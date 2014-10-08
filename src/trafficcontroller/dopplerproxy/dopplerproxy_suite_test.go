package dopplerproxy_test

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"trafficcontroller/listener"
)

func TestDopplerProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerProxy Suite")
}

func assertAuthorizationError(resp *http.Response, err error, msg string) {
	Expect(err).To(HaveOccurred())

	respBody := make([]byte, 4096)
	resp.Body.Read(respBody)
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	Expect(string(respBody)).To(ContainSubstring(msg))
}

func assertResourceNotFoundError(resp *http.Response, err error, msg string) {
	Expect(err).To(HaveOccurred())

	respBody := make([]byte, 128)
	resp.Body.Read(respBody)
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	Expect(string(respBody)).To(ContainSubstring(msg))
}

func websocketClientWithQueryParamAuth(ts *httptest.Server, path string, auth string) ([]byte, *http.Response, error) {
	authorizationParams := ""
	if auth != "" {
		authorizationParams = "&authorization=" + url.QueryEscape(auth)
	}
	url := fmt.Sprintf("ws://%s%s%s", ts.Listener.Addr(), path, authorizationParams)
	return dialConnection(url, http.Header{})
}

func websocketClientWithHeaderAuth(ts *httptest.Server, path string, auth string) ([]byte, *http.Response, error) {
	url := fmt.Sprintf("ws://%s%s", ts.Listener.Addr(), path)
	headers := http.Header{"Authorization": []string{auth}}
	return dialConnection(url, headers)
}

func dialConnection(url string, headers http.Header) ([]byte, *http.Response, error) {
	ws, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err == nil {
		defer ws.Close()
	}
	return clientWithAuth(ws), resp, err
}

func clientWithAuth(ws *websocket.Conn) []byte {
	if ws == nil {
		return nil
	}
	_, data, err := ws.ReadMessage()

	if err != nil {
		ws.Close()
		return nil
	}
	return data
}

type fakeListener struct {
	messageChan  chan []byte
	closed       bool
	startCount   int
	expectedHost string
	sync.Mutex
}

func (fl *fakeListener) Start(host string, appId string, o listener.OutputChannel, s listener.StopChannel) error {
	defer GinkgoRecover()
	fl.Lock()

	fl.startCount += 1
	if fl.expectedHost != "" {
		Expect(host).To(Equal(fl.expectedHost))
	}
	fl.Unlock()

	for {
		select {
		case <-s:
			return nil
		case msg, ok := <-fl.messageChan:
			if !ok {
				return nil
			}
			o <- msg
		}
	}
}

func (fl *fakeListener) Close() {
	fl.Lock()
	defer fl.Unlock()
	if fl.closed {
		return
	}
	fl.closed = true
	close(fl.messageChan)
}

func (fl *fakeListener) IsStarted() bool {
	fl.Lock()
	defer fl.Unlock()
	return fl.startCount > 0
}

func (fl *fakeListener) StartCount() int {
	fl.Lock()
	defer fl.Unlock()
	return fl.startCount
}

func (fl *fakeListener) IsClosed() bool {
	fl.Lock()
	defer fl.Unlock()
	return fl.closed
}

func (fl *fakeListener) SetExpectedHost(value string) {
	fl.Lock()
	defer fl.Unlock()
	fl.expectedHost = value
}

type failingListener struct {
	startCount int
	sync.Mutex
}

func (fl *failingListener) Start(string, string, listener.OutputChannel, listener.StopChannel) error {
	fl.Lock()
	defer fl.Unlock()
	fl.startCount += 1
	return errors.New("fail")
}

func (fl *failingListener) StartCount() int {
	fl.Lock()
	defer fl.Unlock()
	return fl.startCount
}

func serverUp(ts *httptest.Server) func() bool {
	return func() bool {
		url := fmt.Sprintf("http://%s", ts.Listener.Addr())
		resp, _ := http.Head(url)
		return resp != nil && resp.StatusCode == http.StatusOK
	}
}
