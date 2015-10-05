package agentlistener_test

import (
	"net"

	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AgentListener", func() {
	var listener agentlistener.AgentListener
	var dataChannel <-chan []byte
	var listenerStopped chan struct{}

	BeforeEach(func() {
		listenerStopped = make(chan struct{})
		loggertesthelper.TestLoggerSink.Clear()

		listener, dataChannel = agentlistener.NewAgentListener("127.0.0.1:3456", loggertesthelper.Logger(), "agentListener")
		go func() {
			listener.Start()
			close(listenerStopped)
		}()

		Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening on port 127.0.0.1:3456"))
	})

	AfterEach(func() {
		listener.Stop()
		<-listenerStopped
	})

	Context("with a listner running", func() {
		It("listens to the socket and forwards log lines", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", "localhost:3456")

			_, err = connection.Write([]byte(expectedData))
			Expect(err).To(BeNil())

			received := <-dataChannel
			Expect(string(received)).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).To(BeNil())

			receivedAgain := <-dataChannel
			Expect(string(receivedAgain)).To(Equal(otherData))
			close(done)
		}, 2)
	})

	Context("dropsonde metric emission", func() {
		BeforeEach(func() {
			fakeEventEmitter.Reset()
			metricBatcher.Reset()
		})

		It("issues intended metrics", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", "localhost:3456")

			_, err = connection.Write([]byte(expectedData))
			Expect(err).To(BeNil())
			<-dataChannel

			_, err = connection.Write([]byte(otherData))
			Expect(err).To(BeNil())
			<-dataChannel

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(2))

			var counterEvents []*events.CounterEvent

			for _, miniEnvelope := range fakeEventEmitter.GetMessages() {
				counterEvents = append(counterEvents, miniEnvelope.Event.(*events.CounterEvent))
			}

			Expect(counterEvents).To(ConsistOf(
				&events.CounterEvent{
					Name:  proto.String("agentListener.receivedMessageCount"),
					Delta: proto.Uint64(2),
				},
				&events.CounterEvent{
					Name:  proto.String("agentListener.receivedByteCount"),
					Delta: proto.Uint64(19),
				},
			))

			close(done)
		})
	})
})
