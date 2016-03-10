package dopplerforwarder_test

type mockRetrier struct {
	RetryCalled chan bool
	RetryInput  struct {
		message chan []byte
	}
	RetryOutput struct {
		ret0 chan error
	}
}

func newMockRetrier() *mockRetrier {
	m := &mockRetrier{}
	m.RetryCalled = make(chan bool, 100)
	m.RetryInput.message = make(chan []byte, 100)
	m.RetryOutput.ret0 = make(chan error, 100)
	return m
}
func (m *mockRetrier) Retry(message []byte) error {
	m.RetryCalled <- true
	m.RetryInput.message <- message
	return <-m.RetryOutput.ret0
}
