package picker_test

type mockByteArrayWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		message chan []byte
	}
	WeightCalled chan bool
	WeightOutput struct {
		ret0 chan int
	}
}

func newMockByteArrayWriter() *mockByteArrayWriter {
	m := &mockByteArrayWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WeightCalled = make(chan bool, 100)
	m.WeightOutput.ret0 = make(chan int, 100)
	return m
}
func (m *mockByteArrayWriter) Write(message []byte) {
	m.WriteCalled <- true
	m.WriteInput.message <- message
}
func (m *mockByteArrayWriter) Weight() int {
	m.WeightCalled <- true
	return <-m.WeightOutput.ret0
}
