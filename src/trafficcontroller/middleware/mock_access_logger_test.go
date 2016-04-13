package middleware_test

import "net/http"

type mockAccessLogger struct {
	LogAccessCalled chan bool
	LogAccessInput  struct {
		Req chan *http.Request
	}
	LogAccessOutput struct {
		Ret0 chan error
	}
}

func newMockAccessLogger() *mockAccessLogger {
	m := &mockAccessLogger{}
	m.LogAccessCalled = make(chan bool, 100)
	m.LogAccessInput.Req = make(chan *http.Request, 100)
	m.LogAccessOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockAccessLogger) LogAccess(req *http.Request) error {
	m.LogAccessCalled <- true
	m.LogAccessInput.Req <- req
	return <-m.LogAccessOutput.Ret0
}
