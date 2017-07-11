package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gorilla/websocket"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketHandler", func() {
	var (
		handler            http.Handler
		fakeResponseWriter *httptest.ResponseRecorder
		messagesChan       chan []byte
		testServer         *httptest.Server
		handlerDone        chan struct{}
		mockSender         *testhelper.SpyMetricClient
		egressMetric       *metricemitter.Counter
	)

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		messagesChan = make(chan []byte, 10)
		mockSender = testhelper.NewMetricClient()
		egressMetric = mockSender.NewCounter("egress")

		handler = proxy.NewWebsocketHandler(
			messagesChan,
			100*time.Millisecond,
			egressMetric,
		)
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
		close(messagesChan)
		Eventually(handlerDone).Should(BeClosed())
	})

	It("should err when websocket upgrade fails", func() {
		resp, err := http.Get(testServer.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		Eventually(handlerDone).Should(BeClosed())
	})

	It("should stop when the client goes away", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		ws.Close()

		handlerDone, messagesChan := handlerDone, messagesChan
		go func() {
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

	It("should stop when the client goes away, even if no messages come", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		//		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
		ws.Close()

		Eventually(handlerDone).Should(BeClosed())
	})

	It("should stop when the client doesn't respond to pings", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		ws.SetPingHandler(func(string) error { return nil })
		go ws.ReadMessage()

		Eventually(handlerDone).Should(BeClosed())
	})

	It("should continue when the client resonds to pings", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		go ws.ReadMessage()

		Consistently(handlerDone, 200*time.Millisecond).ShouldNot(BeClosed())
		close(messagesChan)
		Eventually(handlerDone).Should(BeClosed())
	})

	It("should continue when the client sends old style keepalives", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		go func() {
			for {
				ws.WriteMessage(websocket.TextMessage, []byte("I'm alive!"))
				time.Sleep(100 * time.Millisecond)
			}
		}()
		go ws.ReadMessage()

		Consistently(handlerDone, 200*time.Millisecond).ShouldNot(BeClosed())
		close(messagesChan)
		Eventually(handlerDone).Should(BeClosed())
	})

	It("should send a closing message", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())
		close(messagesChan)
		_, _, err = ws.ReadMessage()
		Expect(err.Error()).To(ContainSubstring("websocket: close 1000"))
		Eventually(handlerDone).Should(BeClosed())
	})

	It("increments an egress counter every time it writes an envelope", func() {
		ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
		Expect(err).NotTo(HaveOccurred())

		messagesChan <- []byte("message")
		close(messagesChan)

		_, _, err = ws.ReadMessage()
		Expect(err).NotTo(HaveOccurred())

		Eventually(egressMetric.GetDelta).Should(Equal(uint64(1)))
		Eventually(handlerDone).Should(BeClosed())
	})

	Context("when the KeepAlive expires", func() {
		It("sends a CloseInternalServerErr frame", func() {
			ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(200 * time.Millisecond) // Longer than  the keepAlive timeout

			_, _, err = ws.ReadMessage()
			Expect(err.Error()).To(ContainSubstring("1008"))
			Expect(err.Error()).To(ContainSubstring("Client did not respond to ping before keep-alive timeout expired."))
			Eventually(handlerDone).Should(BeClosed())
		})

		It("logs an appropriate message", func() {
			_, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(200 * time.Millisecond) // Longer than  the keepAlive timeout
			Eventually(handlerDone).Should(BeClosed())
		})
	})

	Context("when client goes away", func() {
		It("logs and appropriate message", func() {
			ws, _, err := websocket.DefaultDialer.Dial(httpToWs(testServer.URL), nil)
			Expect(err).NotTo(HaveOccurred())

			ws.Close()
			Eventually(handlerDone).Should(BeClosed())
		})
	})
})

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}
