package websocketserver_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/websocketserver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// WebsocketSinks are a deprecated code path
var _ = XDescribe("WebsocketKeepalive", func() {
	var (
		testServer         *httptest.Server
		wsClient           *websocket.Conn
		keepAliveCompleted chan struct{}
	)

	BeforeEach(func() {
		keepAliveCompleted = make(chan struct{})
		testServer = httptest.NewServer(makeTestHandler(keepAliveCompleted))
		f := func() error {
			var err error
			wsClient, _, err = websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
			return err
		}
		Eventually(f).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		wsClient.Close()
		testServer.Close()
	})

	It("sends pings to the client", func() {
		var pingCount int32
		wsClient.SetPingHandler(func(string) error {
			atomic.AddInt32(&pingCount, 1)
			return nil
		})

		go wsClient.ReadMessage()
		Eventually(func() int32 { return atomic.LoadInt32(&pingCount) }).ShouldNot(BeZero())
	})

	It("doesn't close the client when it responds with pong frames", func() {
		go wsClient.ReadMessage()
		// default ping handler responds with pong frames
		Consistently(keepAliveCompleted).ShouldNot(BeClosed())
	})

	It("closes the client when it doesn't respond with pong frames", func() {
		wsClient.SetPingHandler(func(string) error { return nil })
		go wsClient.ReadMessage()
		Eventually(keepAliveCompleted).Should(BeClosed())
	})
})

func makeTestHandler(keepAliveCompleted chan struct{}) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		conn, _ := websocket.Upgrade(rw, req, nil, 0, 0)
		go conn.ReadMessage()
		websocketserver.NewKeepAlive(conn, 50*time.Millisecond).Run()
		close(keepAliveCompleted)
	})
}

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}
