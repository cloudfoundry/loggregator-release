package networkreader_test

import (
	"fmt"
	"net"

	"metron/networkreader"
	"metron/writers/mocks"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
)

var _ = Describe("NetworkReader", func() {
	var reader *networkreader.NetworkReader
	var readerStopped chan struct{}
	var writer mocks.MockByteArrayWriter
	var port int
	var address string

	BeforeEach(func() {
		port = 3456 + GinkgoParallelNode()
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		writer = mocks.MockByteArrayWriter{}
		reader = networkreader.New(address, "networkReader", &writer, loggertesthelper.Logger())
		readerStopped = make(chan struct{})
	})

	Context("without a running listener", func() {
		It("Emit returns a context with the given name", func() {
			context := reader.Emit()

			Expect(context.Name).To(Equal("networkReader"))
		})
	})

	Context("with a reader running", func() {
		BeforeEach(func() {
			loggertesthelper.TestLoggerSink.Clear()
			go func () {
				reader.Start()
				close(readerStopped)
			}()

			expectedLog := fmt.Sprintf("Listening on port %s", address)
			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring(expectedLog))
		})

		AfterEach(func() {
			reader.Stop()
			<- readerStopped
		})

		It("sends data recieved on UDP socket to its writer", func() {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", address)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			Eventually(writer.Data).Should(HaveLen(1))
			data := string(writer.Data()[0])
			Expect(data).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			Eventually(writer.Data).Should(HaveLen(2))

			data = string(writer.Data()[1])
			Expect(data).To(Equal(otherData))
		})

		It("emits metrics related to data sent in on udp connection", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"
			connection, err := net.Dial("udp", address)
			dataByteCount := len(otherData + expectedData)

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			Eventually(writer.Data).Should(HaveLen(2))

			metrics := reader.Emit().Metrics
			Expect(metrics).To(HaveLen(2))
			for _, metric := range metrics {
				switch metric.Name {
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
