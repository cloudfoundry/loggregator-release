package dopplerforwarder_test

import (
	"metron/writers/dopplerforwarder"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
)

type mockNetworkWrapper struct {
	WriteCalled chan bool
	WriteInput  struct {
		Client   chan dopplerforwarder.Client
		Message  chan []byte
		Chainers chan []metricbatcher.BatchCounterChainer
	}
	WriteOutput struct {
		Ret0 chan error
	}
}

func newMockNetworkWrapper() *mockNetworkWrapper {
	m := &mockNetworkWrapper{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Client = make(chan dopplerforwarder.Client, 100)
	m.WriteInput.Message = make(chan []byte, 100)
	m.WriteInput.Chainers = make(chan []metricbatcher.BatchCounterChainer, 100)
	m.WriteOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockNetworkWrapper) Write(client dopplerforwarder.Client, message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	m.WriteCalled <- true
	m.WriteInput.Client <- client
	m.WriteInput.Message <- message
	m.WriteInput.Chainers <- chainers
	return <-m.WriteOutput.Ret0
}
