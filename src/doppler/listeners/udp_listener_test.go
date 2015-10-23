package listeners_test

import (
	"doppler/listeners"
	"net"
	"strconv"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var _ = Describe("AgentListener", func() {
	var listener listeners.Listener
	var dataChannel <-chan []byte
	var listenerStopped chan struct{}
	var address string

	BeforeEach(func() {
		listenerStopped = make(chan struct{})
		loggertesthelper.TestLoggerSink.Clear()

		port := 3456 + config.GinkgoConfig.ParallelNode
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		listener, dataChannel = listeners.NewAgentListener(address, loggertesthelper.Logger(), "agentListener")
		go func() {
			listener.Start()
			close(listenerStopped)
		}()

		Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening on port " + address))
	})

	AfterEach(func() {
		listener.Stop()
		Eventually(listenerStopped).Should(BeClosed())
	})

	Context("with a listner running", func() {
		It("listens to the socket and forwards log lines", func() {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", address)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).To(BeNil())

			var received []byte
			Eventually(dataChannel).Should(Receive(&received))
			Expect(string(received)).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).To(BeNil())

			Eventually(dataChannel).Should(Receive(&received))
			Expect(string(received)).To(Equal(otherData))
		})
	})

	Context("dropsonde metric emission", func() {
		BeforeEach(func() {
			fakeEventEmitter.Reset()
			metricBatcher.Reset()
		})

		It("issues intended metrics", func() {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", address)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())
			Eventually(dataChannel).Should(Receive())

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())
			Eventually(dataChannel).Should(Receive())

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
		})
	})
})
