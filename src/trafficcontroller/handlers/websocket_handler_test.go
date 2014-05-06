package handlers_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"time"
	"trafficcontroller/handlers"
)

var _ = Describe("WebsocketHandler", func() {
	var handler http.Handler
	var fakeResponseWriter *httptest.ResponseRecorder
	var messagesChan chan []byte
	var testServer *httptest.Server
	var handlerDone chan struct{}

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		messagesChan = make(chan []byte, 10)
		handler = handlers.NewWebsocketHandler(messagesChan, loggertesthelper.Logger(), 100*time.Millisecond)
		handlerDone = make(chan struct{})
		testServer = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(rw, r)
			close(handlerDone)
		}))
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("should complete when the input channel is closed", func() {
		_, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		close(messagesChan)
		Eventually(handlerDone).Should(BeClosed())
	})

	It("fowards messages from the messagesChan to the ws client", func() {
		for i := 0; i < 5; i++ {
			messagesChan <- []byte("message")
		}

		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		for i := 0; i < 5; i++ {
			msgType, msg, err := ws.ReadMessage()
			Expect(msgType).To(Equal(websocket.BinaryMessage))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(msg)).To(Equal("message"))
		}
		go ws.ReadMessage()
		Consistently(handlerDone, 200*time.Millisecond).ShouldNot(BeClosed())
		close(messagesChan)
	})

	It("should err when websocket upgrade fails", func() {
		resp, err := http.Get(testServer.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

	})

	It("should stop when the client goes away", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		ws.Close()
		go func() {
			handlerDone := handlerDone
			for {
				select {
				case messagesChan <- []byte("message"):
				case <-handlerDone:
					return
				}
			}
		}()

		Eventually(handlerDone).Should(BeClosed())
	})

	It("should stop when the client doesn't respond to pings", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		ws.SetPingHandler(func(string) error { return nil })
		go ws.ReadMessage()

		Eventually(handlerDone).Should(BeClosed())
	})

})

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}
