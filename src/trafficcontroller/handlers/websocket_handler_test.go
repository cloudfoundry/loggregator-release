package handlers_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"trafficcontroller/handlers"
)

var _ = Describe("HttpHandler", func() {
	var handler http.Handler
	var fakeResponseWriter *httptest.ResponseRecorder
	var messagesChan chan []byte
	var testServer *httptest.Server

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		messagesChan = make(chan []byte, 10)
		handler = handlers.NewWebsocketHandler(messagesChan, loggertesthelper.Logger())
		testServer = httptest.NewServer(handler)
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("fowards messages from the messagesChan to the ws client", func(done Done) {
		for i := 0; i < 5; i++ {
			messagesChan <- []byte("message")
		}
		close(messagesChan)

		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		for i := 0; i < 5; i++ {
			msgType, msg, err := ws.ReadMessage()
			Expect(msgType).To(Equal(websocket.BinaryMessage))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(msg)).To(Equal("message"))
		}
		close(done)
	})

	It("should err when websocket upgrade fails", func() {
		resp, err := http.Get(testServer.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

	})

})

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}
