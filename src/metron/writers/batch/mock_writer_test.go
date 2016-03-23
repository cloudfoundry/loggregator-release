package batch_test

type mockWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		P chan []byte
	}
	WriteOutput struct {
		N   chan int
		Err chan error
	}
}

func newMockWriter() *mockWriter {
	m := &mockWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.P = make(chan []byte, 100)
	m.WriteOutput.N = make(chan int, 100)
	m.WriteOutput.Err = make(chan error, 100)
	return m
}
func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.WriteCalled <- true
	m.WriteInput.P <- p
	return <-m.WriteOutput.N, <-m.WriteOutput.Err
}
