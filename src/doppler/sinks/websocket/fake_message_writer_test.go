package websocket_test

import (
	"net"
	"sync"
	"time"
)

type fakeMessageWriter struct {
	messages      [][]byte
	writeDeadline time.Time
	sync.RWMutex
	writeMessageErr error
}

func (fake *fakeMessageWriter) RemoteAddr() net.Addr {
	return fakeAddr{}
}

func (fake *fakeMessageWriter) WriteMessage(messageType int, data []byte) error {
	fake.Lock()
	defer fake.Unlock()

	fake.messages = append(fake.messages, data)
	return fake.writeMessageErr
}

func (fake *fakeMessageWriter) SetWriteDeadline(t time.Time) error {
	fake.Lock()
	defer fake.Unlock()
	fake.writeDeadline = t
	return nil
}

func (fake *fakeMessageWriter) ReadMessages() [][]byte {
	fake.RLock()
	defer fake.RUnlock()

	return fake.messages
}

func (fake *fakeMessageWriter) WriteDeadline() time.Time {
	fake.RLock()
	defer fake.RUnlock()
	return fake.writeDeadline
}
