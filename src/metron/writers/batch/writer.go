package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/gosteno"
)

// TODO: move write lock into dopplerForwarder
//	w.writerLock.Lock()
//	defer w.writerLock.Unlock()

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
	asyncWriter   AsyncRetryWriter
	msgBuffer     *messageBuffer
	msgBufferLock sync.Mutex
	timer         *time.Timer
	timerLock     sync.Mutex
	logger        *gosteno.Logger
}

//go:generate hel --type ByteWriter --output mock_writer_test.go

type ByteWriter interface {
	Write(p []byte) (n int, err error)
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
		msgBuffer:     newMessageBuffer(make([]byte, 0, bufferCapacity)),
		timer:         batchTimer,
		logger:        logger,
	}

	batchWriter.asyncWriter = NewAsyncRetryWriter(writer, batchWriter, logger)
	go batchWriter.flushOnTimer()
	return batchWriter, nil
}

func (w *Writer) Write(msgBytes []byte) (int, error) {
	prefixedMessage := w.prefixMessage(msgBytes)

	w.msgBufferLock.Lock()
	defer w.msgBufferLock.Unlock()

	// msgBuffer is full we need to flush
	if w.msgBuffer.Len()+len(prefixedMessage) > w.msgBuffer.Cap() {
		w.flushWrite(prefixedMessage)
		return len(prefixedMessage), nil
	}

	if w.msgBuffer.Len() == 0 {
		w.timerLock.Lock()
		w.timer.Reset(w.flushDuration)
		w.timerLock.Unlock()
	}
	return w.msgBuffer.Write(prefixedMessage)
}

func (w *Writer) Stop() {
	w.timer.Stop()
}

// when calling flushWrite you must have the msgBufferLock acquired
func (w *Writer) flushWrite(bytes []byte) {
	toWrite := make([]byte, 0, w.msgBuffer.Len()+len(bytes))
	toWrite = append(toWrite, w.msgBuffer.Bytes()...)
	toWrite = append(toWrite, bytes...)

	bufferMessageCount := w.msgBuffer.messages
	if len(bytes) > 0 {
		bufferMessageCount++
	}
	w.asyncWriter.AsyncWrite(toWrite, maxOverflowTries, bufferMessageCount)
	w.msgBuffer.Reset()
}

func (w *Writer) flushOnTimer() {
	w.timerLock.Lock()
	w.timer.Reset(w.flushDuration)
	w.timerLock.Unlock()
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
	w.flushWrite(nil)
}

func (w *Writer) prefixMessage(message []byte) []byte {
	tmpBuffer := bytes.NewBuffer(make([]byte, 0, len(message)*2))
	err := binary.Write(tmpBuffer, binary.LittleEndian, uint32(len(message)))
	if err != nil {
		w.logger.Fatalf("Error writing message prefix into temp buffer: %s\n", err)
	}
	_, err = tmpBuffer.Write(message)
	if err != nil {
		w.logger.Fatalf("Error writing message into temp buffer: %s\n", err)
	}
	return tmpBuffer.Bytes()
}
