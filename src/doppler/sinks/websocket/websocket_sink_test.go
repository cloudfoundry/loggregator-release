package websocket_test

import (
	"doppler/sinks/websocket"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"net"
	"sync"

	"code.google.com/p/gogoprotobuf/proto"
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

	Describe("StreamId", func() {
		It("returns the application id", func() {
			Expect(websocketSink.StreamId()).To(Equal("appId"))
		})
	})

	Describe("ShouldReceiveErrors", func() {
		It("returns true", func() {
			Expect(websocketSink.ShouldReceiveErrors()).To(BeTrue())
		})
	})

	Describe("Run", func() {
		var inputChan chan *events.Envelope

		BeforeEach(func() {
			inputChan = make(chan *events.Envelope, 10)
		})

		It("forwards messages", func(done Done) {
			defer close(done)
			go websocketSink.Run(inputChan)

			message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "hello world", "appId", "App"), "origin")
			messageBytes, _ := proto.Marshal(message)

			inputChan <- message
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(1))
			Expect(fakeWebsocket.ReadMessages()[0]).To(Equal(messageBytes))

			messageTwo, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "goodbye world", "appId", "App"), "origin")
			messageTwoBytes, _ := proto.Marshal(messageTwo)
			inputChan <- messageTwo
			Eventually(fakeWebsocket.ReadMessages).Should(HaveLen(2))
			Expect(fakeWebsocket.ReadMessages()[1]).To(Equal(messageTwoBytes))
		})
	})
})
