package batch_test

type mockByteWriter struct {
	WriteCalled chan bool
	WriteInput  struct {
		bytes chan []byte
	}
}

func newMockByteWriter() *mockByteWriter {
	m := &mockByteWriter{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.bytes = make(chan []byte, 100)
	return m
}
func (m *mockByteWriter) Write(bytes []byte) {
	m.WriteCalled <- true
	m.WriteInput.bytes <- bytes
}
