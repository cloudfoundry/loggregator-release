package mocks

import "sync"

type MockByteArrayWriter struct {
	data [][]byte
	lock sync.RWMutex
}

func (m *MockByteArrayWriter) Write(p []byte) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.data = append(m.data, p)
}

func (m *MockByteArrayWriter) Data() [][]byte {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.data
}

func (m *MockByteArrayWriter) Weight() int {
	return 0
}
