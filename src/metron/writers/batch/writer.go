package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const (
	maxOverflowTries  = 5
	minBufferCapacity = 1024
)

type messageBuffer struct {
	bytes.Buffer
	messages uint64
}

func newMessageBuffer(bufferBytes []byte) *messageBuffer {
	child := bytes.NewBuffer(bufferBytes)
	return &messageBuffer{
		Buffer: *child,
	}
}

// Write writes msg to b and increments b.messages.
func (b *messageBuffer) Write(msg []byte) (int, error) {
	b.messages++
	return b.Buffer.Write(msg)
}

// writeNonMessage is provided as a method to write bytes without
// incrementing b.messages.
func (b *messageBuffer) writeNonMessage(msg []byte) (int, error) {
	return b.Buffer.Write(msg)
}

func (b *messageBuffer) Reset() {
	b.messages = 0
	b.Buffer.Reset()
}

//go:generate hel --type BatchChainByteWriter --output mock_byte_writer_test.go

type BatchChainByteWriter interface {
	Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (sentLength int, err error)
}

type Writer struct {
	flushDuration   time.Duration
	outWriter       BatchChainByteWriter
	writerLock      sync.Mutex
	msgBuffer       *messageBuffer
	msgBufferLock   sync.Mutex
	timer           *time.Timer
	logger          *gosteno.Logger
	droppedMessages uint64
	chainers        []metricbatcher.BatchCounterChainer
}

func NewWriter(writer BatchChainByteWriter, bufferCapacity uint64, flushDuration time.Duration, logger *gosteno.Logger) (*Writer, error) {
	if bufferCapacity < minBufferCapacity {
		return nil, fmt.Errorf("batch.Writer requires a buffer of at least %d bytes", minBufferCapacity)
	}

	// Initialize the timer with a long duration so we can stop it before
	// it triggers.  Ideally, we'd initialize the timer without starting
	// it, but that doesn't seem possible in the current library.
	batchTimer := time.NewTimer(time.Second)
	batchTimer.Stop()
	batchWriter := &Writer{
		flushDuration: flushDuration,
		outWriter:     writer,
		msgBuffer:     newMessageBuffer(make([]byte, 0, bufferCapacity)),
		timer:         batchTimer,
		logger:        logger,
	}
	go batchWriter.flushOnTimer()
	return batchWriter, nil
}

func (w *Writer) Write(msgBytes []byte, chainers ...metricbatcher.BatchCounterChainer) (int, error) {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()

	w.chainers = append(w.chainers, chainers...)

	prefixedBytes, err := w.prefixMessage(msgBytes)
	if err != nil {
		w.logger.Errorf("Error encoding message length: %v\n", err)
		metrics.BatchIncrementCounter("tls.sendErrorCount")
		return 0, err
	}
	switch {
	case w.msgBuffer.Len()+len(prefixedBytes) > w.msgBuffer.Cap():
		_, err := w.retryWrites(prefixedBytes)
		if err != nil {
			dropped := w.msgBuffer.messages + 1
			atomic.AddUint64(&w.droppedMessages, dropped)
			metrics.BatchAddCounter("MessageBuffer.droppedMessageCount", dropped)
			w.msgBuffer.Reset()

			w.msgBuffer.writeNonMessage(w.droppedLogMessage())
			w.timer.Reset(w.flushDuration)
			return 0, err
		}
		return len(msgBytes), nil
	default:
		if w.msgBuffer.Len() == 0 {
			w.timer.Reset(w.flushDuration)
		}
		_, err := w.msgBuffer.Write(prefixedBytes)
		return len(msgBytes), err
	}
}

func (w *Writer) Stop() {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()
	w.timer.Stop()
}

func (w *Writer) prefixMessage(msgBytes []byte) ([]byte, error) {
	buffer := bytes.NewBuffer(make([]byte, 0, len(msgBytes)*2))
	err := binary.Write(buffer, binary.LittleEndian, uint32(len(msgBytes)))
	if err != nil {
		return nil, err
	}
	_, err = buffer.Write(msgBytes)
	return buffer.Bytes(), err
}

func (w *Writer) flushWrite(bytes []byte) (int, error) {
	w.writerLock.Lock()
	defer w.writerLock.Unlock()

	toWrite := make([]byte, 0, w.msgBuffer.Len()+len(bytes))
	toWrite = append(toWrite, w.msgBuffer.Bytes()...)
	toWrite = append(toWrite, bytes...)

	bufferMessageCount := w.msgBuffer.messages
	if len(bytes) > 0 {
		bufferMessageCount++
	}
	sent, err := w.outWriter.Write(toWrite, w.chainers...)
	if err != nil {
		w.logger.Warnf("Received error while trying to flush TCP bytes: %s", err)
		return 0, err
	}

	metrics.BatchAddCounter("DopplerForwarder.sentMessages", bufferMessageCount)
	atomic.StoreUint64(&w.droppedMessages, 0)
	w.msgBuffer.Reset()
	w.chainers = nil
	return sent, nil
}

func (w *Writer) flushOnTimer() {
	for range w.timer.C {
		w.flushBuffer()
	}
}

func (w *Writer) flushBuffer() {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()
	if w.msgBuffer.Len() == 0 {
		return
	}
	if _, err := w.flushWrite(nil); err != nil {
		metrics.BatchIncrementCounter("DopplerForwarder.retryCount")
		w.timer.Reset(w.flushDuration)
	}
}

func (w *Writer) retryWrites(message []byte) (sent int, err error) {
	for i := 0; i < maxOverflowTries; i++ {
		if i > 0 {
			metrics.BatchIncrementCounter("DopplerForwarder.retryCount")
		}
		sent, err = w.flushWrite(message)
		if err == nil {
			return sent, nil
		}
	}
	return 0, err
}

func (w *Writer) droppedLogMessage() []byte {
	droppedMessages := atomic.LoadUint64(&w.droppedMessages)
	logMessage := &events.LogMessage{
		Message:     []byte(fmt.Sprintf("Dropped %d message(s) from MetronAgent to Doppler", droppedMessages)),
		MessageType: events.LogMessage_ERR.Enum(),
		AppId:       proto.String(envelope_extensions.SystemAppId),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
	}
	env, err := emitter.Wrap(logMessage, "MetronAgent")
	if err != nil {
		w.logger.Fatalf("Failed to emitter.Wrap a log message: %s", err)
	}
	marshaled, err := proto.Marshal(env)
	if err != nil {
		w.logger.Fatalf("Failed to marshal generated dropped log message: %s", err)
	}
	prefixedBytes, err := w.prefixMessage(marshaled)
	if err != nil {
		w.logger.Fatalf("Failed to prefix dropped log message: %s", err)
	}

	return prefixedBytes
}
