package v1

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
)

const (
	maxOverflowTries  = 5
	minBufferCapacity = 1024
)

type messageBuffer struct {
	buffer   []byte
	messages uint64
}

func newMessageBuffer(bufferBytes []byte) *messageBuffer {
	return &messageBuffer{
		buffer: bufferBytes,
	}
}

// Write writes msg to b and increments b.messages.
func (b *messageBuffer) Write(msg []byte) {
	b.messages++
	b.buffer = append(b.buffer, msg...)
}

func (b *messageBuffer) Reset() {
	b.messages = 0
	b.buffer = b.buffer[:0]
}

//go:generate hel --type DroppedMessageCounter --output mock_dropped_message_counter_test.go

type DroppedMessageCounter interface {
	Drop(count uint32)
}

type Writer struct {
	flushDuration  time.Duration
	outWriter      BatchChainByteWriter
	msgBuffer      *messageBuffer
	msgBufferLock  sync.Mutex
	flushing       sync.WaitGroup
	timer          *time.Timer
	droppedCounter DroppedMessageCounter
	chainers       []metricbatcher.BatchCounterChainer
}

func NewWriter(writer BatchChainByteWriter, droppedCounter DroppedMessageCounter, bufferCapacity uint64, flushDuration time.Duration) (*Writer, error) {
	if bufferCapacity < minBufferCapacity {
		return nil, fmt.Errorf("batch.Writer requires a buffer of at least %d bytes", minBufferCapacity)
	}

	// Initialize the timer with a long duration so we can stop it before
	// it triggers.  Ideally, we'd initialize the timer without starting
	// it, but that doesn't seem possible in the current library.
	batchTimer := time.NewTimer(time.Second)
	batchTimer.Stop()
	batchWriter := &Writer{
		flushDuration:  flushDuration,
		outWriter:      writer,
		droppedCounter: droppedCounter,
		msgBuffer:      newMessageBuffer(make([]byte, 0, bufferCapacity)),
		timer:          batchTimer,
	}
	go batchWriter.flushOnTimer()
	return batchWriter, nil
}

func (w *Writer) Write(msgBytes []byte, chainers ...metricbatcher.BatchCounterChainer) (int, error) {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()

	w.chainers = append(w.chainers, chainers...)

	prefixedBytes := prefixMessage(msgBytes)
	switch {
	case len(w.msgBuffer.buffer)+len(prefixedBytes) > cap(w.msgBuffer.buffer):
		count := w.msgBuffer.messages + 1 // don't forget the message we're trying to write now
		allMsgs := make([]byte, len(w.msgBuffer.buffer), len(w.msgBuffer.buffer)+len(prefixedBytes))
		copy(allMsgs, w.msgBuffer.buffer)
		allMsgs = append(allMsgs, prefixedBytes...)
		chainers := w.chainers

		w.msgBuffer.Reset()
		w.chainers = nil

		// The batch writer should not block the calling context during a flush -
		// it should continue to accept messages into its newly-empty buffer, so
		// kick off the flush in a goroutine
		go func() {
			retryErr := w.retryWrites(allMsgs, count, chainers...)
			if retryErr != nil {
				w.droppedCounter.Drop(uint32(count))
			}
		}()
		return len(msgBytes), nil
	default:
		if w.msgBuffer.messages == 0 {
			w.timer.Reset(w.flushDuration)
		}
		w.msgBuffer.Write(prefixedBytes)
		return len(msgBytes), nil
	}
}

func (w *Writer) Stop() {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()
	w.timer.Stop()
	w.flushing.Wait()
}

func (w *Writer) flushWrite(toWrite []byte, messageCount uint64, chainers ...metricbatcher.BatchCounterChainer) error {
	err := w.outWriter.Write(toWrite, chainers...)
	if err != nil {
		log.Printf("Received error while trying to flush TCP bytes: %s", err)
		return err
	}

	// metric-documentation-v1: (DopplerForwarder.sentMessages) The number of
	// envelopes sent to dopplers v1 API over all protocols
	metrics.BatchAddCounter("DopplerForwarder.sentMessages", messageCount)

	// metric-documentation-v1: (grpc.sentMessageCount) The number of
	// envelopes sent to dopplers v1 gRPC API
	metrics.BatchAddCounter("grpc.sentMessageCount", messageCount)
	return nil
}

func (w *Writer) flushOnTimer() {
	for range w.timer.C {
		w.flushBuffer()
	}
}

func (w *Writer) flushBuffer() {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()
	w.flushing.Add(1)
	defer w.flushing.Done()
	if w.msgBuffer.messages == 0 {
		return
	}
	count := w.msgBuffer.messages
	messages := make([]byte, len(w.msgBuffer.buffer))
	copy(messages, w.msgBuffer.buffer)
	if err := w.flushWrite(messages, count, w.chainers...); err != nil {
		// metric-documentation-v1: (DopplerForwarder.retryCount) Number of
		// retries made when trying to write to Doppler's v1 gRPC API
		metrics.BatchIncrementCounter("DopplerForwarder.retryCount")
		w.timer.Reset(w.flushDuration)
		return
	}
	w.msgBuffer.Reset()
	w.chainers = nil
}

func (w *Writer) retryWrites(message []byte, messageCount uint64, chainers ...metricbatcher.BatchCounterChainer) (err error) {
	w.flushing.Add(1)
	defer w.flushing.Done()
	for i := 0; i < maxOverflowTries; i++ {
		if i > 0 {
			// metric-documentation-v1: (DopplerForwarder.retryCount) Number
			// of retries made when trying to write to Doppler's v1 gRPC API
			metrics.BatchIncrementCounter("DopplerForwarder.retryCount")
		}
		err = w.flushWrite(message, messageCount, chainers...)
		if err == nil {
			return nil
		}
	}
	return err
}

func prefixMessage(msgBytes []byte) []byte {
	prefixed := make([]byte, 4, len(msgBytes)+4)
	binary.LittleEndian.PutUint32(prefixed, uint32(len(msgBytes)))
	return append(prefixed, msgBytes...)
}
