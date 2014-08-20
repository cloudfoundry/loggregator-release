package outputproxy_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"
	"trafficcontroller/listener"
	"trafficcontroller/outputproxy"
	testhelpers "trafficcontroller_testhelpers"
)

var _ = Describe("OutputProxySingleHasher", func() {

	var fwsh *fakeWebsocketHandler
	var outputMessages <-chan []byte
	var fl *fakeListener
	var ts *httptest.Server
	var existingWsProvider = outputproxy.NewWebsocketHandlerProvider
	var fakeLoggregatorProvider *fakeLoggregatorServerProvider

	BeforeEach(func() {
		fwsh = &fakeWebsocketHandler{}
		fl = &fakeListener{messageChan: make(chan []byte)}

		outputMessages = nil
		outputproxy.NewWebsocketHandlerProvider = func(messageChan <-chan []byte) http.Handler {
			outputMessages = messageChan
			return fwsh
		}

		outputproxy.NewWebsocketListener = func() listener.Listener {
			return fl
		}

		fakeLoggregatorProvider = &fakeLoggregatorServerProvider{serverAddresses: []string{"localhost:62038"}}
		proxy := outputproxy.NewProxy(
			fakeLoggregatorProvider,
			testhelpers.SuccessfulAuthorizer,
			cfcomponent.Config{},
			loggertesthelper.Logger(),
		)

		ts = httptest.NewServer(proxy)
		Eventually(serverUp(ts)).Should(BeTrue())
		outputproxy.CheckLoggregatorServersInterval = 10 * time.Millisecond
		outputproxy.WebsocketKeepAliveDuration = time.Second
	})

	AfterEach(func() {
		ts.Close()
		if outputMessages != nil {
			Eventually(outputMessages).Should(BeClosed())
		}
		outputproxy.NewWebsocketHandlerProvider = existingWsProvider
	})

	Context("Auth Params", func() {
		It("should NOT authenticate with Auth Query Params", func() {
			_, resp, err := websocketClientWithQueryParamAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			assertAuthorizationError(resp, err, "Error: Authorization not provided")
		})
	})

	Context("Auth in cookie", func() {
		It("should Authenticate with cookie Params", func() {
			_, resp, _ := websocketClientWithCookieAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			Expect(resp.StatusCode).NotTo(Equal(http.StatusUnauthorized))
		})

		Context("when auth fails", func() {
			It("should Fail to Authenticate with Incorrect Auth Query Params", func() {
				_, resp, err := websocketClientWithCookieAuth(ts, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})

			It("should Fail to Authenticate without Authorization", func() {
				_, resp, err := websocketClientWithCookieAuth(ts, "/?app=myApp", "")
				assertAuthorizationError(resp, err, "Error: Authorization not provided")
			})

			It("should Fail With Invalid Target", func() {
				_, resp, err := websocketClientWithCookieAuth(ts, "/invalid_target", testhelpers.VALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid target")
			})
		})
	})

	Context("Auth Headers", func() {
		It("should Authenticate with Correct Auth Header", func() {
			websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			authenticateHeader := req.Header.Get("Authorization")
			Expect(authenticateHeader).To(Equal(testhelpers.VALID_AUTHENTICATION_TOKEN))

		})

		Context("when auth fails", func() {
			It("should Fail to Authenticate with Incorrect Auth Header", func() {
				_, resp, err := websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})
		})
	})

	Context("when a connection to a loggregator server fails", func() {
		var failingWsListener *failingListener

		BeforeEach(func() {
			failingWsListener = &failingListener{}
			outputproxy.NewWebsocketListener = func() listener.Listener {
				return failingWsListener
			}
		})

		It("should return an error message to the client", func(done Done) {
			websocketClientWithHeaderAuth(ts, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			msgData, ok := <-outputMessages
			Expect(ok).To(BeTrue())
			msg, err := logmessage.ParseMessage(msgData)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.GetLogMessage().GetSourceName()).To(Equal("LGR"))
			Expect(string(msg.GetLogMessage().GetMessage())).To(Equal("proxy: error connecting to a loggregator server"))
			close(done)
		})

		It("should periodically retry to connect to the loggregator server", func() {
			outputproxy.NewWebsocketHandlerProvider = func(messageChan <-chan []byte) http.Handler {
				outputMessages = messageChan
				return existingWsProvider(messageChan)
			}

			url := fmt.Sprintf("ws://%s%s", ts.Listener.Addr(), "/tail/?app=myApp")
			headers := http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			ws, _, err := websocket.DefaultDialer.Dial(url, headers)
			Expect(err).ToNot(HaveOccurred())
			defer ws.Close()

			Eventually(failingWsListener.StartCount).Should(BeNumerically(">", 1))
		})
	})

	Context("when a new loggregator server comes up after the client is connected", func() {
		It("should connect to the new loggregator server", func() {
			outputproxy.NewWebsocketHandlerProvider = func(messageChan <-chan []byte) http.Handler {
				outputMessages = messageChan
				return existingWsProvider(messageChan)
			}
			fakeLoggregatorProvider.serverAddresses = []string{}

			url := fmt.Sprintf("ws://%s%s", ts.Listener.Addr(), "/tail/?app=myApp")
			headers := http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			ws, _, err := websocket.DefaultDialer.Dial(url, headers)
			Expect(err).ToNot(HaveOccurred())
			defer ws.Close()

			Eventually(fakeLoggregatorProvider.CallCount).Should(BeNumerically(">", 0))

			fl.SetExpectedHost("ws://fake-server/tail/?app=myApp")
			Expect(fl.IsStarted()).To(BeFalse())
			fakeLoggregatorProvider.SetServerAddresses([]string{"fake-server"})
			Eventually(fl.IsStarted, 1*time.Second).Should(BeTrue())
		})
	})

	Context("websocket client", func() {
		It("should use the WebsocketHandler for tail", func() {
			fl.SetExpectedHost("ws://localhost:62038/tail/?app=myApp")
			websocketClientWithHeaderAuth(ts, "/tail/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fwsh.called).To(BeTrue())
		})

		It("should use the WebsocketHandler for dump", func() {
			fl.SetExpectedHost("ws://localhost:62038/dump/?app=myApp")
			websocketClientWithHeaderAuth(ts, "/dump/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fwsh.called).To(BeTrue())
		})
	})

	Context("/recent", func() {
		var fhh *fakeHttpHandler
		var originalNewHttpHandlerProvider func(messages <-chan []byte) http.Handler

		BeforeEach(func() {
			fhh = &fakeHttpHandler{}
			fl.SetExpectedHost("ws://localhost:62038/recent?app=myApp")
			originalNewHttpHandlerProvider = outputproxy.NewHttpHandlerProvider
			outputproxy.NewHttpHandlerProvider = func(messageChan <-chan []byte) http.Handler {
				outputMessages = messageChan
				return fhh
			}
		})

		AfterEach(func() {
			outputproxy.NewHttpHandlerProvider = originalNewHttpHandlerProvider
		})

		It("uses HttpHandler instead of WebsocketHandler", func() {
			url := fmt.Sprintf("http://%s/recent?app=myApp", ts.Listener.Addr())
			r, _ := http.NewRequest("GET", url, nil)
			r.Header = http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			client := &http.Client{}
			_, err := client.Do(r)

			Expect(err).NotTo(HaveOccurred())
			Expect(fhh.called).To(BeTrue())
		})

		It("closes the connection", func() {
			outputproxy.NewHttpHandlerProvider = func(messageChan <-chan []byte) http.Handler {
				outputMessages = messageChan
				return originalNewHttpHandlerProvider(messageChan)
			}

			fakeLoggregatorProvider.serverAddresses = []string{}
			url := fmt.Sprintf("http://%s/recent?app=myApp", ts.Listener.Addr())
			r, _ := http.NewRequest("GET", url, nil)
			r.Header = http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			client := &http.Client{}
			_, err := client.Do(r)
			Expect(err).ToNot(HaveOccurred())
			Expect(outputMessages).To(BeClosed())
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

func websocketClientWithCookieAuth(ts *httptest.Server, path string, auth string) ([]byte, *http.Response, error) {
	u := fmt.Sprintf("ws://%s%s", ts.Listener.Addr(), path)

	h := http.Header{}
	h.Add("Cookie", "authorization="+url.QueryEscape(auth))
	return dialConnection(u, h)
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

type fakeLoggregatorServerProvider struct {
	serverAddresses []string
	callCount       int
	sync.Mutex
}

func (p *fakeLoggregatorServerProvider) CallCount() int {
	p.Lock()
	defer p.Unlock()
	return p.callCount
}

func (p *fakeLoggregatorServerProvider) SetServerAddresses(addresses []string) {
	p.Lock()
	defer p.Unlock()
	p.serverAddresses = addresses
}

func (p *fakeLoggregatorServerProvider) LoggregatorServerAddresses() []string {
	p.Lock()
	defer p.Unlock()
	p.callCount += 1
	return p.serverAddresses
}
