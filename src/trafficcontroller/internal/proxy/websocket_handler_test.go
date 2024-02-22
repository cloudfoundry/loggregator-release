package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	"code.cloudfoundry.org/loggregator-release/src/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketHandler", func() {
	var (
		input       chan []byte
		count       *metricemitter.Counter
		keepAlive   time.Duration
		handlerDone chan struct{}
		ts          *httptest.Server
		conn        *websocket.Conn
		wc          *websocketClient
	)

	BeforeEach(func() {
		input = make(chan []byte, 10)
		keepAlive = 200 * time.Millisecond
		count = metricemitter.NewCounter("egress", "")
		wc = newWebsocketClient()
		wsh := proxy.NewWebsocketHandler(input, keepAlive, count)
		handlerDone = make(chan struct{})
		ts = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			done := handlerDone
			wsh.ServeHTTP(rw, r)
			close(done)
		}))
		DeferCleanup(ts.Close)

		u, err := url.Parse(ts.URL)
		Expect(err).NotTo(HaveOccurred())
		u.Scheme = "ws"
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		Expect(err).NotTo(HaveOccurred())
		conn = c
	})

	JustBeforeEach(func() {
		go wc.Start(conn)
	})

	AfterEach(func() {
		select {
		case _, ok := <-input:
			if ok {
				close(input)
			}
		default:
			close(input)
		}
		<-wc.Done
	})

	It("forwards byte arrays from the input channel to the websocket client", func() {
		go func() {
			for i := 0; i < 10; i++ {
				input <- []byte("testing")
			}
		}()

		type websocketResp struct {
			messageType int
			message     string
		}
		expectedResp := websocketResp{messageType: websocket.BinaryMessage, message: "testing"}
		for i := 0; i < 10; i++ {
			Eventually(func() (websocketResp, error) {
				msgType, msg, ok := wc.Read()
				if !ok {
					err, _ := wc.ReadError()
					return websocketResp{}, err
				}
				return websocketResp{messageType: msgType, message: msg}, nil
			}).Should(Equal(expectedResp))
		}
	})

	Context("when the input channel is closed", func() {
		JustBeforeEach(func() {
			close(input)
		})

		It("stops", func() {
			Eventually(handlerDone, keepAlive/2).Should(BeClosed())
		})

		It("closes the connection", func() {
			Eventually(wc.Done, keepAlive/2).Should(BeClosed())
			Eventually(func() error {
				err, _ := wc.ReadError()
				return err
			}).Should(MatchError(&websocket.CloseError{
				Code: websocket.CloseNormalClosure,
				Text: "",
			}))
		})
	})

	It("does not accept http requests", func() {
		resp, err := http.Get(ts.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	Context("when the client closes the connection", func() {
		JustBeforeEach(func() {
			conn.Close()
		})

		It("stops", func() {
			Eventually(handlerDone, keepAlive/2).Should(BeClosed())
		})
	})

	Context("when the client doesn't respond to pings for the keep-alive duration", func() {
		BeforeEach(func() {
			conn.SetPingHandler(func(string) error { return nil })
		})

		It("stops", func() {
			Eventually(handlerDone).Should(BeClosed())
		})

		It("closes the connection with a ClosePolicyViolation", func() {
			Eventually(wc.Done).Should(BeClosed())
			Eventually(func() error {
				err, _ := wc.ReadError()
				return err
			}).Should(MatchError(&websocket.CloseError{
				Code: websocket.ClosePolicyViolation,
				Text: "Client did not respond to ping before keep-alive timeout expired.",
			}))
		})
	})

	Context("when the client responds to pings", func() {
		It("does not stop", func() {
			Consistently(handlerDone, keepAlive*2).ShouldNot(BeClosed())
		})

		It("does not close the connection", func() {
			Consistently(wc.Done, keepAlive*2).ShouldNot(BeClosed())
		})
	})

	It("keeps a count of every time it writes an envelope", func() {
		Expect(count.GetDelta()).To(Equal(uint64(0)))
		input <- []byte("message")
		Eventually(count.GetDelta).Should(Equal(uint64(1)))
	})
})

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}

type websocketClient struct {
	mu sync.Mutex

	Done chan struct{}

	readError       []error
	readMessageType []int
	readMessage     []string
}

func newWebsocketClient() *websocketClient {
	return &websocketClient{
		Done: make(chan struct{}),
	}
}

func (wc *websocketClient) Start(conn *websocket.Conn) {
	defer conn.Close()
	defer close(wc.Done)
	for {
		messageType, message, err := conn.ReadMessage()
		wc.mu.Lock()
		if err != nil {
			wc.readError = append(wc.readError, err)
			wc.mu.Unlock()
			return
		}
		wc.readMessageType = append(wc.readMessageType, messageType)
		wc.readMessage = append(wc.readMessage, string(message))
		wc.mu.Unlock()
	}
}

func (wc *websocketClient) ReadError() (error, bool) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if len(wc.readError) == 0 {
		return nil, false
	}
	err := wc.readError[0]
	wc.readError = wc.readError[1:]
	return err, true
}

func (wc *websocketClient) Read() (messageType int, message string, ok bool) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if len(wc.readMessageType) == 0 {
		return 0, "", false
	}
	ok = true
	messageType = wc.readMessageType[0]
	wc.readMessageType = wc.readMessageType[1:]
	message = wc.readMessage[0]
	wc.readMessage = wc.readMessage[1:]
	return
}

func (wc *websocketClient) Write() (messageType int, message string, ok bool) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if len(wc.readMessageType) == 0 {
		return 0, "", false
	}
	ok = true
	messageType = wc.readMessageType[0]
	wc.readMessageType = wc.readMessageType[1:]
	message = wc.readMessage[0]
	wc.readMessage = wc.readMessage[1:]
	return
}
