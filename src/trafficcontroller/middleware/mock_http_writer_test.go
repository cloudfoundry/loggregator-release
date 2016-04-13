package middleware_test

import "net/http"

type mockHttpHandler struct {
	ServeHTTPCalled chan bool
	ServeHTTPInput  struct {
		Arg0 chan http.ResponseWriter
		Arg1 chan *http.Request
	}
}

func newMockHttpHandler() *mockHttpHandler {
	m := &mockHttpHandler{}
	m.ServeHTTPCalled = make(chan bool, 100)
	m.ServeHTTPInput.Arg0 = make(chan http.ResponseWriter, 100)
	m.ServeHTTPInput.Arg1 = make(chan *http.Request, 100)
	return m
}
func (m *mockHttpHandler) ServeHTTP(arg0 http.ResponseWriter, arg1 *http.Request) {
	m.ServeHTTPCalled <- true
	m.ServeHTTPInput.Arg0 <- arg0
	m.ServeHTTPInput.Arg1 <- arg1
}
