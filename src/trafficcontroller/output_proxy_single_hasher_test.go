package trafficcontroller_test

import (
	"errors"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/url"
	"time"
	"trafficcontroller"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
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
	var PORT = "62022"
	var proxy *trafficcontroller.Proxy
	var outputMessages <-chan []byte

	BeforeEach(func() {
		fwsh = &fakeWebsocketHandler{}
		trafficcontroller.NewWebsocketHandlerProvider = func(messageChan <-chan []byte, logger *gosteno.Logger) http.Handler {
			outputMessages = messageChan
			return fwsh
		}

		hashers = []hasher.Hasher{hasher.NewHasher([]string{"localhost:62038"})}
		proxy = trafficcontroller.NewProxy(
			"localhost:"+PORT,
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)
		go proxy.Start()
	})

	AfterEach(func() {
		proxy.Stop()
	})

	Context("Auth Params", func() {
		It("Should Authenticate with Auth Query Params", func() {
			websocketClientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			Expect(req.URL.RawQuery).To(Equal("app=myApp&authorization=bearer+correctAuthorizationToken"))
		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Query Params", func() {
				_, resp, err := websocketClientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})

			It("Should Fail to Authenticate without Authorization", func() {
				_, resp, err := websocketClientWithQueryParamAuth(PORT, "/?app=myApp", "")
				assertAuthorizationError(resp, err, "Error: Authorization not provided")
			})

			It("Should Fail With Invalid Target", func() {
				_, resp, err := websocketClientWithQueryParamAuth(PORT, "/invalid_target", testhelpers.VALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid target")
			})
		})

	})

	Context("Auth Headers", func() {
		It("Should Authenticate with Correct Auth Header", func() {
			websocketClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			authenticateHeader := req.Header.Get("Authorization")
			Expect(authenticateHeader).To(Equal(testhelpers.VALID_AUTHENTICATION_TOKEN))

		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Header", func() {
				_, resp, err := websocketClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})
		})
	})

	Context("when a connection to a loggregator server fails", func() {
		BeforeEach(func() {
			hashers = []hasher.Hasher{newFailingHasher([]string{"localhost:62038"})}
		})

		It("should return an error message to the client", func(done Done) {
			websocketClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			msgData := <-outputMessages
			msg, _ := logmessage.ParseMessage(msgData)
			Expect(msg.GetLogMessage().GetSourceName()).To(Equal("LGR"))
			Expect(string(msg.GetLogMessage().GetMessage())).To(Equal("proxy: error connecting to a loggregator server"))
			close(done)
		})
	})

	Context("websocket client", func() {

		It("should use the WebsocketHandler", func() {
			websocketClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fwsh.called).To(BeTrue())
		})

	})

	Context("/recent", func() {
		var fhh *fakeHttpHandler

		BeforeEach(func() {
			fhh = &fakeHttpHandler{}

			trafficcontroller.NewHttpHandlerProvider = func(<-chan []byte, *gosteno.Logger) http.Handler {
				return fhh
			}
		})

		It("should use HttpHandler instead of WebsocketHandler", func() {
			url := "http://localhost:" + PORT + "/recent?app=myApp"
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

func websocketClientWithQueryParamAuth(port string, path string, auth string) ([]byte, *http.Response, error) {
	authorizationParams := ""
	if auth != "" {
		authorizationParams = "&authorization=" + url.QueryEscape(auth)
	}
	url := "ws://localhost:" + port + path + authorizationParams

	return dialConnection(url, http.Header{})
}

func websocketClientWithHeaderAuth(port string, path string, auth string) ([]byte, *http.Response, error) {
	url := "ws://localhost:" + port + path
	headers := http.Header{"Authorization": []string{auth}}
	return dialConnection(url, headers)
}

func dialConnection(url string, headers http.Header) ([]byte, *http.Response, error) {
	numTries := 0
	for {
		ws, resp, err := websocket.DefaultDialer.Dial(url, headers)
		if err != nil {
			numTries++
			if numTries == 5 {
				return nil, resp, err
			}
			time.Sleep(1)
			continue
		} else {
			defer ws.Close()
			return clientWithAuth(ws), resp, nil
		}
	}
}

func clientWithAuth(ws *websocket.Conn) []byte {
	_, data, err := ws.ReadMessage()

	if err != nil {
		ws.Close()
		return nil
	}
	return data
}

type failingHasher struct {
	servers []string
}

func (fh *failingHasher) LoggregatorServers() []string {
	return fh.servers
}
func (fh *failingHasher) GetLoggregatorServerForAppId(string) string {
	return fh.servers[0]
}
func (fh *failingHasher) ProxyMessagesFor(appId string, out listener.OutputChannel, stop listener.StopChannel) error {
	return errors.New("connection failed")
}

func newFailingHasher(servers []string) hasher.Hasher {
	return &failingHasher{servers}
}
