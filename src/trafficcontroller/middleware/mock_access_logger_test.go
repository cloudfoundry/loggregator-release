package middleware_test

import "net/http"

type mockAccessLogger struct {
	LogAccessCalled chan bool
	LogAccessInput  struct {
		Req  chan *http.Request
		Host chan string
		Port chan uint32
	}
	LogAccessOutput struct {
		Ret0 chan error
	}
}

func newMockAccessLogger() *mockAccessLogger {
	m := &mockAccessLogger{}
	m.LogAccessCalled = make(chan bool, 100)
	m.LogAccessInput.Req = make(chan *http.Request, 100)
	m.LogAccessInput.Host = make(chan string, 100)
	m.LogAccessInput.Port = make(chan uint32, 100)
	m.LogAccessOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockAccessLogger) LogAccess(req *http.Request, host string, port uint32) error {
	m.LogAccessCalled <- true
	m.LogAccessInput.Req <- req
	m.LogAccessInput.Host <- host
	m.LogAccessInput.Port <- port
	return <-m.LogAccessOutput.Ret0
}
