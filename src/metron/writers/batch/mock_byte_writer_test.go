package batch_test

type mockByteWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		message chan []byte
	}
	WriteOutput struct {
		sentLength chan int
		err        chan error
	}
}

func newMockByteWriter() *mockByteWriter {
	m := &mockByteWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WriteOutput.sentLength = make(chan int, 100)
	m.WriteOutput.err = make(chan error, 100)
	return m
}
func (m *mockByteWriter) Write(message []byte) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.message <- message
	return <-m.WriteOutput.sentLength, <-m.WriteOutput.err
}
