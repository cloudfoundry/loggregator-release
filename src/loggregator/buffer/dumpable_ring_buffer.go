package buffer

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"sync"
)

type DumpableRingBuffer struct {
	data       []*logmessage.Message
	closed     chan bool
	outChannel chan *logmessage.Message
	bufferSize int
	sync.RWMutex
}

func NewDumpableRingBuffer(in <-chan *logmessage.Message, bufferSize int) *DumpableRingBuffer {
	rb := new(DumpableRingBuffer)
	rb.bufferSize = bufferSize
	rb.data = make([]*logmessage.Message, 0, bufferSize)
	rb.outChannel = make(chan *logmessage.Message, bufferSize)

	rb.closed = make(chan bool)
	go func() {
		for m := range in {
			rb.addData(m)
			select {
			case rb.outChannel <- m:
			default:
				<-rb.outChannel
				rb.outChannel <- m
			}
		}
		close(rb.closed)
		close(rb.outChannel)
	}()
	return rb
}

func (r *DumpableRingBuffer) WaitForClose() {
	<-r.closed
}

func (r *DumpableRingBuffer) Dump(outChan chan<- *logmessage.Message) {
	for _, m := range r.copyData() {
		outChan <- m
	}
	close(outChan)
}

func (r *DumpableRingBuffer) OutputChannel() <-chan *logmessage.Message {
	return r.outChannel
}

func (r *DumpableRingBuffer) addData(m *logmessage.Message) {
	r.Lock()
	defer r.Unlock()
	if len(r.data) == r.bufferSize {
		r.data = append(r.data[1:], m)
	} else {
		r.data = append(r.data, m)
	}
}

func (r *DumpableRingBuffer) copyData() []*logmessage.Message {
	r.RLock()
	defer r.RUnlock()
	result := make([]*logmessage.Message, len(r.data))
	copy(result, r.data)
	return result
}
