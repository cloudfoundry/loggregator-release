package clientpool_test

type mockClient struct {
	SchemeCalled chan bool
	SchemeOutput struct {
		ret0 chan string
	}
	AddressCalled chan bool
	AddressOutput struct {
		ret0 chan string
	}
	WriteCalled chan bool
	WriteInput  struct {
		message chan []byte
	}
	WriteOutput struct {
		bytesSent chan int
		err       chan error
	}
	CloseCalled chan bool
	CloseOutput struct {
		ret0 chan error
	}
}

func newMockClient() *mockClient {
	m := &mockClient{}
	m.SchemeCalled = make(chan bool, 100)
	m.SchemeOutput.ret0 = make(chan string, 100)
	m.AddressCalled = make(chan bool, 100)
	m.AddressOutput.ret0 = make(chan string, 100)
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.message = make(chan []byte, 100)
	m.WriteOutput.bytesSent = make(chan int, 100)
	m.WriteOutput.err = make(chan error, 100)
	m.CloseCalled = make(chan bool, 100)
	m.CloseOutput.ret0 = make(chan error, 100)
	return m
}
func (m *mockClient) Scheme() string {
	m.SchemeCalled <- true
	return <-m.SchemeOutput.ret0
}
func (m *mockClient) Address() string {
	m.AddressCalled <- true
	return <-m.AddressOutput.ret0
}
func (m *mockClient) Write(message []byte) (bytesSent int, err error) {
	m.WriteCalled <- true
	m.WriteInput.message <- message
	return <-m.WriteOutput.bytesSent, <-m.WriteOutput.err
}
func (m *mockClient) Close() error {
	m.CloseCalled <- true
	return <-m.CloseOutput.ret0
}
