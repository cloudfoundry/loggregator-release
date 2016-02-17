package picker_test

type mockWeightedByteWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		message chan []byte
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

func newMockWeightedByteWriter() *mockWeightedByteWriter {
	m := &mockWeightedByteWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WriteOutput.sentLength = make(chan int, 100)
	m.WriteOutput.err = make(chan error, 100)
	m.WeightCalled = make(chan bool, 100)
	m.WeightOutput.ret0 = make(chan int, 100)
	return m
}
func (m *mockWeightedByteWriter) Write(message []byte) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.message <- message
	return <-m.WriteOutput.sentLength, <-m.WriteOutput.err
}
func (m *mockWeightedByteWriter) Weight() int {
	m.WeightCalled <- true
	return <-m.WeightOutput.ret0
}
