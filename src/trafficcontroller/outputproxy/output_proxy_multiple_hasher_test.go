package outputproxy_test

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
	"trafficcontroller/outputproxy"
	testhelpers "trafficcontroller_testhelpers"
)

var _ = Describe("OutputProxyMultipleHasher", func() {

	var hashers []hasher.Hasher

	var fls []*fakeListener
	var count int
	var ts *httptest.Server

	BeforeEach(func() {
		count = 0

		hashers = []hasher.Hasher{
			hasher.NewHasher([]string{"localhost:62038"}),
			hasher.NewHasher([]string{"localhost:62039"}),
		}

		fls = []*fakeListener{
			&fakeListener{messageChan: make(chan []byte, 1), expectedHost: "ws://" + hashers[0].LoggregatorServers()[0] + "/tail/?app=myApp"},
			&fakeListener{messageChan: make(chan []byte, 1), expectedHost: "ws://" + hashers[1].LoggregatorServers()[0] + "/tail/?app=myApp"},
		}

		proxy := outputproxy.NewProxy(
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)

		outputproxy.NewWebsocketListener = func() listener.Listener {
			defer func() { count++ }()
			return fls[count]
		}

		ts = httptest.NewServer(proxy)
		Eventually(serverUp(ts)).Should(BeTrue())
	})

	AfterEach(func() {
		ts.Close()
	})

	Describe("Listeners", func() {

		It("should listen to all servers", func() {
			fls[0].messageChan <- []byte("data")
			websocketClientWithHeaderAuth(ts, "/tail/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Eventually(fls[0].IsStarted).Should(BeTrue())
			Eventually(fls[1].IsStarted).Should(BeTrue())
			fls[0].Close()
			fls[1].Close()
		})

		It("should close the message chan once all listeners are done", func(done Done) {
			url := fmt.Sprintf("ws://%s/tail/?app=myApp", ts.Listener.Addr())
			headers := http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			ws, _, err := websocket.DefaultDialer.Dial(url, headers)
			Expect(err).NotTo(HaveOccurred())
			go func() {
				_, _, err := ws.ReadMessage()
				Expect(err).To(HaveOccurred())
				close(done)
			}()
			fls[0].Close()
			fls[1].Close()
		})
	})
})
