package outputproxy_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
	"trafficcontroller/outputproxy"
	testhelpers "trafficcontroller_testhelpers"
)

type fakeWebsocketHandler struct {
	called             bool
	lastResponseWriter http.ResponseWriter
	lastRequest        *http.Request
}

func (f *fakeWebsocketHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	f.called = true
	f.lastRequest = r
	f.lastResponseWriter = rw
}

type fakeHttpHandler struct {
	called bool
}

func (f *fakeHttpHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	f.called = true
}

var _ = Describe("OutputProxySingleHasher", func() {

	var fwsh *fakeWebsocketHandler
	var hashers []hasher.Hasher
	var outputMessages <-chan []byte
	var fl *fakeListener
	var ts *httptest.Server
	var existingWsProvider = outputproxy.NewWebsocketHandlerProvider

	BeforeEach(func() {
		fwsh = &fakeWebsocketHandler{}
		fl = &fakeListener{messageChan: make(chan []byte)}

		outputproxy.NewWebsocketHandlerProvider = func(messageChan <-chan []byte) http.Handler {
			outputMessages = messageChan
			return fwsh
		}

		outputproxy.NewWebsocketListener = func() listener.Listener {
			return fl
		}

		hashers = []hasher.Hasher{hasher.NewHasher([]string{"localhost:62038", "localhost:72038"})}
		proxy := outputproxy.NewProxy(
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)

		ts = httptest.NewServer(proxy)
		Eventually(serverUp(ts)).Should(BeTrue())
	})

	AfterEach(func() {
		ts.Close()
		outputproxy.NewWebsocketHandlerProvider = existingWsProvider
	})

	Context("Auth Params", func() {
		It("Should Authenticate with Auth Query Params", func() {
			websocketClientWithQueryParamAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			Expect(req.URL.RawQuery).To(Equal("app=myApp&authorization=bearer+correctAuthorizationToken"))
		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Query Params", func() {
				_, resp, err := websocketClientWithQueryParamAuth(ts, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})

			It("Should Fail to Authenticate without Authorization", func() {
				_, resp, err := websocketClientWithQueryParamAuth(ts, "/?app=myApp", "")
				assertAuthorizationError(resp, err, "Error: Authorization not provided")
			})

			It("Should Fail With Invalid Target", func() {
				_, resp, err := websocketClientWithQueryParamAuth(ts, "/invalid_target", testhelpers.VALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid target")
			})
		})

	})

	Context("Auth Headers", func() {
		It("Should Authenticate with Correct Auth Header", func() {
			websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			authenticateHeader := req.Header.Get("Authorization")
			Expect(authenticateHeader).To(Equal(testhelpers.VALID_AUTHENTICATION_TOKEN))

		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Header", func() {
				_, resp, err := websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})
		})
	})

	Context("when a connection to a loggregator server fails", func() {
		BeforeEach(func() {
			outputproxy.NewWebsocketListener = func() listener.Listener {
				return &failingListener{}
			}
		})

		It("should return an error message to the client", func(done Done) {
			websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			msgData := <-outputMessages
			msg, _ := logmessage.ParseMessage(msgData)
			Expect(msg.GetLogMessage().GetSourceName()).To(Equal("LGR"))
			Expect(string(msg.GetLogMessage().GetMessage())).To(Equal("proxy: error connecting to a loggregator server"))
			close(done)
		})
	})

	Context("websocket client", func() {

		It("should use the WebsocketHandler for tail", func() {
			fl.SetExpectedHosts("ws://localhost:62038/tail/?app=myApp", "ws://localhost:72038/tail/?app=myApp")
			websocketClientWithHeaderAuth(ts, "/tail/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fl.RemainingExpectedHosts()).To(BeEmpty())
			Expect(fwsh.called).To(BeTrue())
		})

		It("should use the WebsocketHandler for dump", func() {
			fl.SetExpectedHosts("ws://localhost:62038/dump/?app=myApp", "ws://localhost:72038/dump/?app=myApp")
			websocketClientWithHeaderAuth(ts, "/dump/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fl.RemainingExpectedHosts()).To(BeEmpty())
			Expect(fwsh.called).To(BeTrue())
		})

	})

	Context("/recent", func() {
		var fhh *fakeHttpHandler

		BeforeEach(func() {
			fhh = &fakeHttpHandler{}
			fl.SetExpectedHosts("ws://localhost:62038/recent?app=myApp", "ws://localhost:72038/recent?app=myApp")
			outputproxy.NewHttpHandlerProvider = func(<-chan []byte) http.Handler {
				return fhh
			}
		})

		It("should use HttpHandler instead of WebsocketHandler", func() {
			url := fmt.Sprintf("http://%s/recent?app=myApp", ts.Listener.Addr())
			r, _ := http.NewRequest("GET", url, nil)
			r.Header = http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			client := &http.Client{}
			_, err := client.Do(r)

			Expect(err).NotTo(HaveOccurred())
			Expect(fhh.called).To(BeTrue())
		})
	})
})

func assertAuthorizationError(resp *http.Response, err error, msg string) {
	Expect(err).To(HaveOccurred())

	respBody := make([]byte, 4096)
	resp.Body.Read(respBody)
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
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
	messageChan            chan []byte
	started, closed        bool
	remainingExpectedHosts []string
	sync.Mutex
}

func (fl *fakeListener) Start(host string, appId string, o listener.OutputChannel, s listener.StopChannel) error {
	defer GinkgoRecover()
	fl.Lock()

	fl.started = true

	if fl.remainingExpectedHosts != nil {
		Expect(fl.remainingExpectedHosts).To(ContainElement(host))
		fl.remainingExpectedHosts = removeElement(fl.remainingExpectedHosts, host)
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

func removeElement(items []string, item string) []string {
	for i, value := range items {
		if value == item {
			return append(items[:i], items[i+1:]...)
		}
	}

	return items
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
	return fl.started
}

func (fl *fakeListener) IsClosed() bool {
	fl.Lock()
	defer fl.Unlock()
	return fl.closed
}

func (fl *fakeListener) SetExpectedHosts(value ...string) {
	fl.Lock()
	defer fl.Unlock()
	fl.remainingExpectedHosts = value
}

func (fl *fakeListener) RemainingExpectedHosts() []string {
	fl.Lock()
	defer fl.Unlock()
	return fl.remainingExpectedHosts
}

type failingListener struct{}

func (fl *failingListener) Start(string, string, listener.OutputChannel, listener.StopChannel) error {
	return errors.New("fail")
}

func serverUp(ts *httptest.Server) func() bool {
	return func() bool {
		url := fmt.Sprintf("http://%s", ts.Listener.Addr())
		resp, _ := http.Head(url)
		return resp != nil && resp.StatusCode == http.StatusOK
	}
}
