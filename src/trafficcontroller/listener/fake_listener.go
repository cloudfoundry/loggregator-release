package listener

import (
	"sync"
)

type FakeListener struct {
	messageChan chan []byte
	closed      bool
	startCount  int
	host        string
	startError  error
	stopped     bool
	readError   error
	sync.Mutex
}

func NewFakeListener(messageChan chan []byte, startError error) *FakeListener {
	return &FakeListener{messageChan: messageChan, startError: startError}
}

func (listener *FakeListener) Start(host string, appId string, outChan OutputChannel, stopChan StopChannel) error {
	if listener.startError != nil {
		return listener.startError
	}

	listener.Lock()

	listener.startCount += 1
	listener.host = host
	listener.Unlock()

	for {
		if listener.readError != nil {
			err := listener.readError
			listener.readError = nil
			return err
		}

		select {
		case <-stopChan:
			listener.stopped = true
			return nil
		case msg, ok := <-listener.messageChan:
			if !ok {
				return nil
			}
			outChan <- msg
		}
	}
}

func (listener *FakeListener) Close() {
	listener.Lock()
	defer listener.Unlock()
	if listener.closed {
		return
	}
	listener.closed = true
	close(listener.messageChan)
}

func (listener *FakeListener) IsStarted() bool {
	listener.Lock()
	defer listener.Unlock()
	return listener.startCount > 0
}

func (listener *FakeListener) StartCount() int {
	listener.Lock()
	defer listener.Unlock()
	return listener.startCount
}

func (listener *FakeListener) IsClosed() bool {
	listener.Lock()
	defer listener.Unlock()
	return listener.closed
}

func (listener *FakeListener) ConnectedHost() string {
	listener.Lock()
	defer listener.Unlock()
	return listener.host
}

func (listener *FakeListener) IsStopped() bool {
	listener.Lock()
	defer listener.Unlock()
	return listener.stopped
}

func (listener *FakeListener) SetReadError(err error) {
	listener.Lock()
	defer listener.Unlock()
	listener.readError = err
}
