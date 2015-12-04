package websocket_test

import (
	"doppler/sinks/websocket"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

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
	messages      [][]byte
	writeDeadline time.Time
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

func (fake *fakeMessageWriter) SetWriteDeadline(t time.Time) error {
	fake.Lock()
	defer fake.Unlock()
	fake.writeDeadline = t
	return nil
}

func (fake *fakeMessageWriter) ReadMessages() [][]byte {
	fake.RLock()
	defer fake.RUnlock()

	return fake.messages
}

func (fake *fakeMessageWriter) WriteDeadline() time.Time {
	fake.RLock()
	defer fake.RUnlock()
	return fake.writeDeadline
}

type fakeCounter struct {
	incrementCalls chan struct{}
}

func (f *fakeCounter) Increment() {
	f.incrementCalls <- struct{}{}
}

var _ = Describe("WebsocketSink", func() {

	var (
		logger        *gosteno.Logger
		websocketSink *websocket.WebsocketSink
		fakeWebsocket *fakeMessageWriter
		writeTimeout  time.Duration
	)

	BeforeEach(func() {
		logger = loggertesthelper.Logger()
		fakeWebsocket = &fakeMessageWriter{}
		writeTimeout = 5 * time.Second
		websocketSink = websocket.NewWebsocketSink("appId", logger, fakeWebsocket, 10, writeTimeout, "dropsonde-origin")
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

		It("sets write deadline", func() {
			go websocketSink.Run(inputChan)
			message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "hello world", "appId", "App"), "origin")
			inputChan <- message
			Eventually(fakeWebsocket.WriteDeadline).Should(BeTemporally("~", time.Now().Add(writeTimeout), time.Millisecond * 5))
		})

		Describe("SetCounter", func() {
			var counter *fakeCounter

			BeforeEach(func() {
				counter = &fakeCounter{
					incrementCalls: make(chan struct{}),
				}
				websocketSink.SetCounter(counter)
			})

			It("uses the passed in counter", func() {
				go websocketSink.Run(inputChan)
				message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "hello world", "appId", "App"), "origin")
				inputChan <- message
				Eventually(counter.incrementCalls).Should(Receive())
				Consistently(counter.incrementCalls).ShouldNot(Receive())
			})
		})
	})
})
