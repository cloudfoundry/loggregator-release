package websocket_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	loghelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinks/websocket"
	"net"
	"sync"
)

type fakeAddr struct{}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}
func (fake fakeAddr) String() string {
	return "RemoteAddressString"
}

type fakeMessageWriter struct {
	messages [][]byte
	sync.RWMutex
}

func (fake *fakeMessageWriter) RemoteAddr() net.Addr {
	return fakeAddr{}
}
func (fake *fakeMessageWriter) WriteMessage(messageType int, data []byte) error {
	fake.Lock()
	defer fake.Unlock()

	fake.messages = append(fake.messages, data)
	return nil
}

func (fake *fakeMessageWriter) ReadMessages() [][]byte {
	fake.RLock()
	defer fake.RUnlock()

	return fake.messages
}

var _ = Describe("WebsocketSink", func() {

	var (
		logger        *gosteno.Logger
		websocketSink *websocket.WebsocketSink
		fakeWebsocket *fakeMessageWriter
	)

	BeforeEach(func() {
		logger = loggertesthelper.Logger()
		fakeWebsocket = &fakeMessageWriter{}
		websocketSink = websocket.NewWebsocketSink("appId", logger, fakeWebsocket, 10)
	})

	Describe("Identifier", func() {
		It("returns the remote address", func() {
			Expect(websocketSink.Identifier()).To(Equal("RemoteAddressString"))
		})
	})

	Describe("AppId", func() {
		It("returns the application id", func() {
			Expect(websocketSink.AppId()).To(Equal("appId"))
		})
	})

	Describe("ShouldReceiveErrors", func() {
		It("returns true", func() {
			Expect(websocketSink.ShouldReceiveErrors()).To(BeTrue())
		})
	})

	Describe("Run", func() {
		var inputChan chan *logmessage.Message

		BeforeEach(func() {
			inputChan = make(chan *logmessage.Message, 10)
		})

		It("forwards messages", func(done Done) {
			defer close(done)
			go websocketSink.Run(inputChan)

			message, _ := loghelpers.NewMessageWithError("hello world", "appId")
			inputChan <- message
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(1))
			Expect(fakeWebsocket.ReadMessages()[0]).To(Equal(message.GetRawMessage()))

			messageTwo, _ := loghelpers.NewMessageWithError("goodbye world", "appId")
			inputChan <- messageTwo
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(2))
			Expect(fakeWebsocket.ReadMessages()[1]).To(Equal(messageTwo.GetRawMessage()))
		})

		It("increments counters", func(done Done) {
			defer close(done)
			go websocketSink.Run(inputChan)

			message, _ := loghelpers.NewMessageWithError("hello world", "appId")
			inputChan <- message
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(1))

			metrics := websocketSink.Emit().Metrics
			Expect(metrics).To(HaveLen(2))

			for _, metric := range metrics {
				switch metric.Name {
				case "sentMessageCount:appId":
					Expect(metric.Value).To(BeNumerically("==", 1))
				case "sentByteCount:appId":
					Expect(metric.Value).To(BeNumerically("==", message.GetRawMessageLength()))
				default:
					Fail("Unexpected metric: " + metric.Name)
				}
			}
		})
	})
})
