package dopplerforwarder_test

type mockClient struct {
	WriteCalled chan bool
	WriteInput  struct {
		message chan []byte
	}
	WriteOutput struct {
		sentLength chan int
		err        chan error
	}
	CloseCalled chan bool
}

func newMockClient() *mockClient {
	m := &mockClient{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WriteOutput.sentLength = make(chan int, 100)
	m.WriteOutput.err = make(chan error, 100)
	m.CloseCalled = make(chan bool, 100)
	return m
}
func (m *mockClient) Write(message []byte) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.message <- message
	return <-m.WriteOutput.sentLength, <-m.WriteOutput.err
}
func (m *mockClient) Close() {
	m.CloseCalled <- true
}
