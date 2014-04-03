package trafficcontroller_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/url"
	"sync"
	"time"
	"trafficcontroller"
	"trafficcontroller/hasher"
	testhelpers "trafficcontroller_testhelpers"
)

type fakeProxyHandler struct {
	sync.Mutex
	callParams          [][]interface{}
	WebsocketConnection *websocket.Conn
}

func (f *fakeProxyHandler) CallParams() [][]interface{} {
	f.Lock()
	defer f.Unlock()
	copyOfParams := make([][]interface{}, len(f.callParams))
	copy(copyOfParams, f.callParams)
	return copyOfParams
}
func (f *fakeProxyHandler) HandleWebSocket(appid, requestUri string, hashers []*hasher.Hasher) {
	f.Lock()
	defer f.Unlock()
	f.callParams = append(f.callParams, []interface{}{appid, requestUri, hashers})
	f.WebsocketConnection.Close()
}

var _ = Describe("OutPutProxy", func() {

	var fph *fakeProxyHandler
	var hashers []*hasher.Hasher
	var PORT = "62022"

	BeforeEach(func() {
		fph = &fakeProxyHandler{callParams: make([][]interface{}, 0)}
		trafficcontroller.NewProxyHandlerProvider = func(ws *websocket.Conn, logger *gosteno.Logger) trafficcontroller.WebsocketHandler {
			fph.WebsocketConnection = ws
			return fph
		}
		hashers = []*hasher.Hasher{hasher.NewHasher([]string{"localhost:62038"})}
		proxy := trafficcontroller.NewProxy(
			"localhost:"+PORT,
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)
		go proxy.Start()
	})

	Context("Auth Params", func() {
		It("Should Authenticate with Auth Query Params", func() {
			clientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			callParams := fph.CallParams()
			Expect(callParams).To(HaveLen(1))
			Expect(callParams[0][0]).To(Equal("myApp"))
			Expect(callParams[0][1]).To(Equal("/?app=myApp&authorization=bearer+correctAuthorizationToken"))
			Expect(callParams[0][2]).To(Equal(hashers))
		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Query Params", func() {
				_, resp, err := clientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})

			It("Should Fail to Authenticate without Authorization", func() {
				_, resp, err := clientWithQueryParamAuth(PORT, "/?app=myApp", "")
				assertAuthorizationError(resp, err, "Error: Authorization not provided")
			})

			It("Should Fail With Invalid Target", func() {
				_, resp, err := clientWithQueryParamAuth(PORT, "/invalid_target", testhelpers.VALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid target")
			})
		})

	})

	Context("Auth Headers", func() {
		It("Should Authenticate with Correct Auth Header", func() {
			clientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			callParams := fph.CallParams()
			Expect(callParams).To(HaveLen(1))
			Expect(callParams[0][0]).To(Equal("myApp"))
			Expect(callParams[0][1]).To(Equal("/?app=myApp"))
			Expect(callParams[0][2]).To(Equal(hashers))
		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Header", func() {
				_, resp, err := clientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})
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

func clientWithQueryParamAuth(port string, path string, auth string) ([]byte, *http.Response, error) {
	authorizationParams := ""
	if auth != "" {
		authorizationParams = "&authorization=" + url.QueryEscape(auth)
	}
	url := "ws://localhost:" + port + path + authorizationParams

	return dialConnection(url, http.Header{})
}

func clientWithHeaderAuth(port string, path string, auth string) ([]byte, *http.Response, error) {
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
