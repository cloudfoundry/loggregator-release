package servertools_test

import (
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"servertools"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

var _ = Describe("WebsocketKeepalive", func() {
	var (
		testServer         *httptest.Server
		wsClient           *websocket.Conn
		keepAliveCompleted chan struct{}
	)

	BeforeEach(func() {
		keepAliveCompleted = make(chan struct{})
		testServer = httptest.NewServer(makeTestHandler(keepAliveCompleted))
		var err error
		wsClient, _, err = websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		go wsClient.ReadMessage()
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

		Eventually(func() int32 { return atomic.LoadInt32(&pingCount) }).ShouldNot(BeZero())
	})

	It("doesn't close the client when it responds with pong frames", func() {
		// default ping handler responds with pong frames
		Consistently(keepAliveCompleted).ShouldNot(BeClosed())
	})

	It("closes the client when it doesn't respond with pong frames", func() {
		wsClient.SetPingHandler(func(string) error { return nil })
		Eventually(keepAliveCompleted).Should(BeClosed())
	})
})

func makeTestHandler(keepAliveCompleted chan struct{}) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		conn, _ := websocket.Upgrade(rw, req, nil, 1000, 1000)
		go conn.ReadMessage()
		servertools.NewKeepAlive(conn, 50*time.Millisecond).Run()
		close(keepAliveCompleted)
	})
}

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}
