package accesslogger_test

type mockWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		Message chan []byte
	}
	WriteOutput struct {
		Sent chan int
		Err  chan error
	}
}

func newMockWriter() *mockWriter {
	m := &mockWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Message = make(chan []byte, 100)
	m.WriteOutput.Sent = make(chan int, 100)
	m.WriteOutput.Err = make(chan error, 100)
	return m
}
func (m *mockWriter) Write(message []byte) (sent int, err error) {
	m.WriteCalled <- true
	m.WriteInput.Message <- message
	return <-m.WriteOutput.Sent, <-m.WriteOutput.Err
}
