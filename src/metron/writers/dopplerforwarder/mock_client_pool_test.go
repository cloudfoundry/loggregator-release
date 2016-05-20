package dopplerforwarder_test

import "metron/writers/dopplerforwarder"

type mockClientPool struct {
	RandomClientCalled chan bool
	RandomClientOutput struct {
		Client chan dopplerforwarder.Client
		Err    chan error
	}
	SizeCalled chan bool
	SizeOutput struct {
		Ret0 chan int
	}
}

func newMockClientPool() *mockClientPool {
	m := &mockClientPool{}
	m.RandomClientCalled = make(chan bool, 100)
	m.RandomClientOutput.Client = make(chan dopplerforwarder.Client, 100)
	m.RandomClientOutput.Err = make(chan error, 100)
	m.SizeCalled = make(chan bool, 100)
	m.SizeOutput.Ret0 = make(chan int, 100)
	return m
}
func (m *mockClientPool) RandomClient() (client dopplerforwarder.Client, err error) {
	m.RandomClientCalled <- true
	return <-m.RandomClientOutput.Client, <-m.RandomClientOutput.Err
}
func (m *mockClientPool) Size() int {
	m.SizeCalled <- true
	return <-m.SizeOutput.Ret0
}
