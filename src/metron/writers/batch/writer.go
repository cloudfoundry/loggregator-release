package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
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

func (b *messageBuffer) Write(msg []byte) (int, error) {
	b.messages++
	return b.Buffer.Write(msg)
}

func (b *messageBuffer) Reset() {
	b.messages = 0
	b.Buffer.Reset()
}

type Writer struct {
	flushDuration time.Duration
	outWriter     ByteWriter
	writerLock    sync.Mutex
	msgBuffer     *messageBuffer
	msgBufferLock sync.Mutex
	timer         *time.Timer
	logger        *gosteno.Logger
}

//go:generate hel --type ByteWriter --output mock_byte_writer_test.go

type ByteWriter interface {
	Write(message []byte) (sentLength int, err error)
}

func NewWriter(writer ByteWriter, bufferCapacity uint64, flushDuration time.Duration, logger *gosteno.Logger) (*Writer, error) {
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

func (w *Writer) Write(msgBytes []byte) (int, error) {
	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()

	buffer := bytes.NewBuffer(make([]byte, 0, len(msgBytes)*2))
	err := binary.Write(buffer, binary.LittleEndian, uint32(len(msgBytes)))
	if err != nil {
		w.logger.Errorf("Error encoding message length: %v\n", err)
		metrics.BatchIncrementCounter("tls.sendErrorCount")
		return 0, err
	}
	_, err = buffer.Write(msgBytes)
	if err != nil {
		return 0, err
	}
	switch {
	case w.msgBuffer.Len()+buffer.Len() > w.msgBuffer.Cap():
		sent, err := w.retryWrites(buffer.Bytes())
		if err != nil {
			dropped := w.msgBuffer.messages + 1
			metrics.BatchAddCounter("MessageBuffer.droppedMessageCount", dropped)
			w.msgBuffer.Reset()
			logMsg, marshalErr := proto.Marshal(w.droppedLogMessage(dropped))
			if marshalErr != nil {
				w.logger.Fatalf("Failed to marshal generated log message: %s", logMsg)
			}

			// w.Write has to be called in a goroutine to allow the defer
			// statement to unlock the mutex lock
			go w.Write(logMsg)
			return 0, err
		}
		return sent, nil
	default:
		if w.msgBuffer.Len() == 0 {
			w.timer.Reset(w.flushDuration)
		}
		return w.msgBuffer.Write(buffer.Bytes())
	}
}

func (w *Writer) Stop() {
	w.timer.Stop()
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
	sent, err := w.outWriter.Write(toWrite)
	if err != nil {
		w.logger.Warnf("Received error while trying to flush TCP bytes: %s", err)
		return 0, err
	}

	metrics.BatchAddCounter("DopplerForwarder.sentMessages", bufferMessageCount)
	w.msgBuffer.Reset()
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
	if w.msgBuffer.messages == 0 {
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

func (w *Writer) droppedLogMessage(droppedMessages uint64) *events.Envelope {
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
	return env
}
