package dopplerforwarder_test

import "metron/writers/dopplerforwarder"

type mockClientPool struct {
	RandomClientCalled chan bool
	RandomClientOutput struct {
		client chan dopplerforwarder.Client
		err    chan error
	}
}

func newMockClientPool() *mockClientPool {
	m := &mockClientPool{}
	m.RandomClientCalled = make(chan bool, 100)
	m.RandomClientOutput.client = make(chan dopplerforwarder.Client, 100)
	m.RandomClientOutput.err = make(chan error, 100)
	return m
}
func (m *mockClientPool) RandomClient() (client dopplerforwarder.Client, err error) {
	m.RandomClientCalled <- true
	return <-m.RandomClientOutput.client, <-m.RandomClientOutput.err
}
