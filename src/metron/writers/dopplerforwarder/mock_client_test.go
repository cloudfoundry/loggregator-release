package dopplerforwarder_test

type mockClient struct {
	WriteCalled chan bool
	WriteInput  struct {
		Message chan []byte
	}
	WriteOutput struct {
		SentLength chan int
		Err        chan error
	}
	CloseCalled chan bool
	CloseOutput struct {
		Ret0 chan error
	}
}

func newMockClient() *mockClient {
	m := &mockClient{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Message = make(chan []byte, 100)
	m.WriteOutput.SentLength = make(chan int, 100)
	m.WriteOutput.Err = make(chan error, 100)
	m.CloseCalled = make(chan bool, 100)
	m.CloseOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockClient) Write(message []byte) (sentLength int, err error) {
	m.WriteCalled <- true
	m.WriteInput.Message <- message
	return <-m.WriteOutput.SentLength, <-m.WriteOutput.Err
}
func (m *mockClient) Close() error {
	m.CloseCalled <- true
	return <-m.CloseOutput.Ret0
}
