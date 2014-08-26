package websocket_test

import (
	"doppler/envelopewrapper"
	"doppler/sinks/websocket"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"net"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		websocketSink = websocket.NewWebsocketSink("appId", logger, fakeWebsocket, 10, "dropsonde-origin")
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
		var inputChan chan *envelopewrapper.WrappedEnvelope

		BeforeEach(func() {
			inputChan = make(chan *envelopewrapper.WrappedEnvelope, 10)
		})

		It("forwards messages", func(done Done) {
			defer close(done)
			go websocketSink.Run(inputChan)

			message, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "hello world", "appId", "App"), "origin")
			inputChan <- message
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(1))
			Expect(fakeWebsocket.ReadMessages()[0]).To(Equal(message.EnvelopeBytes))

			messageTwo, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "goodbye world", "appId", "App"), "origin")
			inputChan <- messageTwo
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(2))
			Expect(fakeWebsocket.ReadMessages()[1]).To(Equal(messageTwo.EnvelopeBytes))
		})

		It("increments counters", func(done Done) {
			defer close(done)
			go websocketSink.Run(inputChan)

			message, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "hello world", "appId", "App"), "origin")
			inputChan <- message
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(1))

			metrics := websocketSink.Emit().Metrics
			Expect(metrics).To(HaveLen(2))

			for _, metric := range metrics {
				switch metric.Name {
				case "sentMessageCount:appId":
					Expect(metric.Value).To(BeNumerically("==", 1))
				case "sentByteCount:appId":
					Expect(metric.Value).To(BeNumerically("==", message.EnvelopeLength()))
				default:
					Fail("Unexpected metric: " + metric.Name)
				}
			}
		})
	})
})
