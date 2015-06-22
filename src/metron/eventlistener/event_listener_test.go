package eventlistener_test

import (
	"fmt"
	"metron/eventlistener"
	"net"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventListener", func() {
	Context("without a running listener", func() {
		It("Emit returns a context with the given name", func() {
			listener, _ := eventlistener.New("127.0.0.1:3456", "secretEventOrange", gosteno.NewLogger("TestLogger"))
			context := listener.Emit()

			Expect(context.Name).To(Equal("secretEventOrange"))
		})
	})

	Context("with a listener running", func() {
		var listener *eventlistener.EventListener
		var dataChannel <-chan []byte
		var listenerClosed chan struct{}

		BeforeEach(func() {
			listenerClosed = make(chan struct{})

			listener, dataChannel = eventlistener.New("127.0.0.1:3456", "eventListener", loggertesthelper.Logger())

			loggertesthelper.TestLoggerSink.Clear()
			go func() {
				listener.Start()
				close(listenerClosed)
			}()
			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening on port 127.0.0.1:3456"))
		})

		AfterEach(func() {
			listener.Stop()
			<-listenerClosed
		})

		It("sends data recieved on UDP socket to the channel", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", "localhost:3456")

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			received := <-dataChannel
			Expect(string(received)).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			receivedAgain := <-dataChannel
			Expect(string(receivedAgain)).To(Equal(otherData))

			close(done)
		}, 5)

		It("emits metrics related to data sent in on udp connection", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"
			connection, err := net.Dial("udp", "localhost:3456")
			dataByteCount := len(otherData + expectedData)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			Eventually(dataChannel).Should(Receive())
			Eventually(dataChannel).Should(Receive())

			metrics := listener.Emit().Metrics
			Expect(metrics).To(HaveLen(3))
			for _, metric := range metrics {
				switch metric.Name {
				case "currentBufferCount":
					Expect(metric.Value).To(Equal(0))
				case "receivedMessageCount":
					Expect(metric.Value).To(Equal(uint64(2)))
				case "receivedByteCount":
					Expect(metric.Value).To(Equal(uint64(dataByteCount)))
				default:
					Fail(fmt.Sprintf("Got an invalid metric name: %s", metric.Name))
				}
			}
			close(done)
		}, 2)
	})
})
