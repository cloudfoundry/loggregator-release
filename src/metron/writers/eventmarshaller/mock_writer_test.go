package eventmarshaller_test

import "github.com/cloudfoundry/dropsonde/metricbatcher"

type mockBatchChainByteWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		Message  chan []byte
		Chainers chan []metricbatcher.BatchCounterChainer
	}
	WriteOutput struct {
		SentLength chan int
		Err        chan error
	}
}

func newMockBatchChainByteWriter() *mockBatchChainByteWriter {
	m := &mockBatchChainByteWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Message = make(chan []byte, 100)
	m.WriteInput.Chainers = make(chan []metricbatcher.BatchCounterChainer, 100)
	m.WriteOutput.SentLength = make(chan int, 100)
	m.WriteOutput.Err = make(chan error, 100)
	return m
}
func (m *mockBatchChainByteWriter) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.Message <- message
	m.WriteInput.Chainers <- chainers
	return <-m.WriteOutput.SentLength, <-m.WriteOutput.Err
}
