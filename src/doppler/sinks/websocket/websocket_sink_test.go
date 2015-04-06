package websocket_test

import (
	"doppler/sinks"
	"doppler/sinks/websocket"
	"net"
	"sync"

	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"doppler/sinks"

	"github.com/gogo/protobuf/proto"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeAddr struct{}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}
func (fake fakeAddr) String() string {
	return "client-address"
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
		logger            *gosteno.Logger
		websocketSink     *websocket.WebsocketSink
		fakeWebsocket     *fakeMessageWriter
		updateMetricsChan = make(chan sinks.DrainMetric)
	)

	BeforeEach(func() {
		logger = loggertesthelper.Logger()
		fakeWebsocket = &fakeMessageWriter{}
		websocketSink = websocket.NewWebsocketSink("appId", logger, fakeWebsocket, 10, "dropsonde-origin", updateMetricsChan)
	})

	Describe("Identifier", func() {
		It("returns the remote address", func() {
			Expect(websocketSink.Identifier()).To(Equal("client-address"))
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

	Describe("UpdateDroppedMessageCount", func() {
		It("updates the dropped message count and sends them to sinkManager metrics", func() {
			drainMetric := sinks.DrainMetric{AppId: "appId", DrainURL: fakeWebsocket.RemoteAddr().String(), DroppedMsgCount: uint64(10)}

			recvMetric := retrieveDroppedMsgCountMetric(websocketSink, updateMetricsChan, 10)
			Expect(*recvMetric).To(Equal(drainMetric))
		})

		It("does not send message if droppedMsgCount is 0", func() {
			Expect(retrieveDroppedMsgCountMetric(websocketSink, updateMetricsChan, 0)).To(BeNil())
		})
	})
})

func retrieveDroppedMsgCountMetric(sink sinks.Sink, updateMetricsChan chan sinks.DrainMetric, messageCount uint64) *sinks.DrainMetric {
	go sink.UpdateDroppedMessageCount(messageCount)

	var recvMetric *sinks.DrainMetric
	ticker := time.NewTicker(500 * time.Millisecond)
	select {
	case metric := <-updateMetricsChan:
		recvMetric = &metric
	case <-ticker.C:
	}
	return recvMetric
}

type fakeAddr struct{}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}
func (fake fakeAddr) String() string {
	return "syslog://using-fake"
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
