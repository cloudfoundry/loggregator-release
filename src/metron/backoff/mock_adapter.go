package backoff

import (
	"errors"
	"sync"
)

type MockAdapter struct {
	connErr error
	sync.Mutex
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{connErr: nil}
}

func (m *MockAdapter) Connect() error {
	m.Lock()
	defer m.Unlock()
	return m.connErr
}

func (m *MockAdapter) Reset() {
	m.Lock()
	defer m.Unlock()
	m.connErr = nil
}

func (m *MockAdapter) ConnectErr(err string) {
	m.Lock()
	defer m.Unlock()
	m.connErr = errors.New(err)
}
