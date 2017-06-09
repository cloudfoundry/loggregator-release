package doppler_test

import "github.com/cloudfoundry/dropsonde/metricbatcher"

type mockEventBatcher struct {
	BatchCounterCalled chan bool
	BatchCounterInput  struct {
		Name chan string
	}
	BatchCounterOutput struct {
		Chainer chan metricbatcher.BatchCounterChainer
	}
	BatchIncrementCounterCalled chan bool
	BatchIncrementCounterInput  struct {
		Name chan string
	}
}

func newMockEventBatcher() *mockEventBatcher {
	m := &mockEventBatcher{}
	m.BatchCounterCalled = make(chan bool, 100)
	m.BatchCounterInput.Name = make(chan string, 100)
	m.BatchCounterOutput.Chainer = make(chan metricbatcher.BatchCounterChainer, 100)
	m.BatchIncrementCounterCalled = make(chan bool, 100)
	m.BatchIncrementCounterInput.Name = make(chan string, 100)
	return m
}
func (m *mockEventBatcher) BatchCounter(name string) (chainer metricbatcher.BatchCounterChainer) {
	m.BatchCounterCalled <- true
	m.BatchCounterInput.Name <- name
	return <-m.BatchCounterOutput.Chainer
}
func (m *mockEventBatcher) BatchIncrementCounter(name string) {
	m.BatchIncrementCounterCalled <- true
	m.BatchIncrementCounterInput.Name <- name
}
