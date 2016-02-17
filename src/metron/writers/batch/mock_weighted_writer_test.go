package batch_test

type mockWeightedWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		bytes chan []byte
	}
	WriteOutput struct {
		sentLength chan int
		err        chan error
	}
	WeightCalled chan bool
	WeightOutput struct {
		ret0 chan int
	}
}

func newMockWeightedWriter() *mockWeightedWriter {
	m := &mockWeightedWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.bytes = make(chan []byte, 100)
	m.WriteOutput.sentLength = make(chan int, 100)
	m.WriteOutput.err = make(chan error, 100)
	m.WeightCalled = make(chan bool, 100)
	m.WeightOutput.ret0 = make(chan int, 100)
	return m
}
func (m *mockWeightedWriter) Write(bytes []byte) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.bytes <- bytes
	return <-m.WriteOutput.sentLength, <-m.WriteOutput.err
}
func (m *mockWeightedWriter) Weight() int {
	m.WeightCalled <- true
	return <-m.WeightOutput.ret0
}
