package trafficcontroller

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"sync"
	"time"
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
		NewProxyHandlerProvider = func(ws *websocket.Conn, logger *gosteno.Logger) websocketHandler {
			fph.WebsocketConnection = ws
			return fph
		}
		hashers = []*hasher.Hasher{hasher.NewHasher([]string{"localhost:62038"})}
		proxy := NewProxy(
			"localhost:"+PORT,
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)
		go proxy.Start()
	})

	Context("Auth Params", func() {
		It("Should Authenticate with Auth Query Params", func() {
			ClientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			callParams := fph.CallParams()
			Expect(callParams).To(HaveLen(1))
			Expect(callParams[0][0]).To(Equal("myApp"))
			Expect(callParams[0][1]).To(Equal("/?app=myApp&authorization=bearer+correctAuthorizationToken"))
			Expect(callParams[0][2]).To(Equal(hashers))
		})

		It("Should Fail to Authenticate with Incorrect Auth Query Params", func() {
			data := ClientWithQueryParamAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)

			receivedMessage := &logmessage.LogMessage{}
			err := proto.Unmarshal(data, receivedMessage)

			Expect(err).To(BeNil())
			Expect(string(receivedMessage.GetMessage())).To(Equal("Error: Invalid authorization"))
			Expect(fph.callParams).To(HaveLen(0))
		})

		It("Should Fail to Authenticate without Authorization", func() {

			data := ClientWithQueryParamAuth(PORT, "/?app=myApp", "")

			receivedMessage := &logmessage.LogMessage{}
			err := proto.Unmarshal(data, receivedMessage)

			Expect(err).To(BeNil())
			Expect(string(receivedMessage.GetMessage())).To(Equal("Error: Authorization not provided"))
			Expect(fph.callParams).To(HaveLen(0))
		})

		It("Should Fail With Invalid Target", func() {

			data := ClientWithQueryParamAuth(PORT, "/invalid_target", testhelpers.VALID_AUTHENTICATION_TOKEN)

			receivedMessage := &logmessage.LogMessage{}
			err := proto.Unmarshal(data, receivedMessage)

			Expect(err).To(BeNil())
			Expect(string(receivedMessage.GetMessage())).To(Equal("Error: Invalid target"))
			Expect(fph.callParams).To(HaveLen(0))
		})

	})

	Context("Auth Headers", func() {
		It("Should Authenticate with Correct Auth Header", func() {
			ClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)

			callParams := fph.CallParams()
			Expect(callParams).To(HaveLen(1))
			Expect(callParams[0][0]).To(Equal("myApp"))
			Expect(callParams[0][1]).To(Equal("/?app=myApp"))
			Expect(callParams[0][2]).To(Equal(hashers))
		})

		It("Should Fail to Authenticate with Incorrect Auth Header", func() {
			data := ClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN)

			receivedMessage := &logmessage.LogMessage{}
			err := proto.Unmarshal(data, receivedMessage)

			Expect(err).To(BeNil())
			Expect(string(receivedMessage.GetMessage())).To(Equal("Error: Invalid authorization"))
			Expect(fph.callParams).To(HaveLen(0))
		})

	})

})

func ClientWithQueryParamAuth(port string, path string, auth string) []byte {
	authorizationParams := ""
	if auth != "" {
		authorizationParams = "&authorization=" + url.QueryEscape(auth)
	}
	url := "ws://localhost:" + port + path + authorizationParams

	return DialConnection(url, http.Header{})
}

func ClientWithHeaderAuth(port string, path string, auth string) []byte {
	url := "ws://localhost:" + port + path
	headers := http.Header{"Authorization": []string{auth}}
	return DialConnection(url, headers)
}

func DialConnection(url string, headers http.Header) []byte {
	numTries := 0
	for {
		ws, _, err := websocket.DefaultDialer.Dial(url, headers)
		if err != nil {
			numTries++
			if numTries == 5 {
				Fail(err.Error())
				return nil
			}
			time.Sleep(1)
			continue
		} else {
			defer ws.Close()
			return ClientWithAuth(ws)
		}
	}
}

func ClientWithAuth(ws *websocket.Conn) []byte {

	_, data, err := ws.ReadMessage()

	if err != nil {
		ws.Close()
		return nil
	}
	return data

}
