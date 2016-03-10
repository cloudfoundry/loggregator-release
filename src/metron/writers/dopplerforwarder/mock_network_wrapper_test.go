package dopplerforwarder_test

import "metron/writers/dopplerforwarder"

type mockNetworkWrapper struct {
	WriteCalled chan bool
	WriteInput  struct {
		client  chan dopplerforwarder.Client
		message chan []byte
	}
	WriteOutput struct {
		ret0 chan error
	}
}

func newMockNetworkWrapper() *mockNetworkWrapper {
	m := &mockNetworkWrapper{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.client = make(chan dopplerforwarder.Client, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WriteOutput.ret0 = make(chan error, 100)
	return m
}
func (m *mockNetworkWrapper) Write(client dopplerforwarder.Client, message []byte) error {
	m.WriteCalled <- true
	m.WriteInput.client <- client
	m.WriteInput.message <- message
	return <-m.WriteOutput.ret0
}
