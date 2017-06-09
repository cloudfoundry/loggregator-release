package doppler_test

import "github.com/cloudfoundry/dropsonde/metricbatcher"

type mockBatchCounterChainer struct {
	SetTagCalled chan bool
	SetTagInput  struct {
		Key, Value chan string
	}
	SetTagOutput struct {
		Ret0 chan metricbatcher.BatchCounterChainer
	}
	IncrementCalled chan bool
	AddCalled       chan bool
	AddInput        struct {
		Value chan uint64
	}
}

func newMockBatchCounterChainer() *mockBatchCounterChainer {
	m := &mockBatchCounterChainer{}
	m.SetTagCalled = make(chan bool, 100)
	m.SetTagInput.Key = make(chan string, 100)
	m.SetTagInput.Value = make(chan string, 100)
	m.SetTagOutput.Ret0 = make(chan metricbatcher.BatchCounterChainer, 100)
	m.IncrementCalled = make(chan bool, 100)
	m.AddCalled = make(chan bool, 100)
	m.AddInput.Value = make(chan uint64, 100)
	return m
}
func (m *mockBatchCounterChainer) SetTag(key, value string) metricbatcher.BatchCounterChainer {
	m.SetTagCalled <- true
	m.SetTagInput.Key <- key
	m.SetTagInput.Value <- value
	return <-m.SetTagOutput.Ret0
}
func (m *mockBatchCounterChainer) Increment() {
	m.IncrementCalled <- true
}
func (m *mockBatchCounterChainer) Add(value uint64) {
	m.AddCalled <- true
	m.AddInput.Value <- value
}
