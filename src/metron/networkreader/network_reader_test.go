package networkreader_test

import (
	"fmt"
	"net"

	"metron/networkreader"
	"metron/writers/mocks"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetworkReader", func() {
	Context("without a running listener", func() {
		It("Emit returns a context with the given name", func() {
			reader := networkreader.New("127.0.0.1:3456", gosteno.NewLogger("TestLogger"), "secretEventOrange", &mocks.MockByteArrayWriter{})
			context := reader.Emit()

			Expect(context.Name).To(Equal("secretEventOrange"))
		})
	})

	Context("with a reader running", func() {
		var reader *networkreader.NetworkReader
		var writer mocks.MockByteArrayWriter

		BeforeEach(func() {
			writer = mocks.MockByteArrayWriter{}
			reader = networkreader.New("127.0.0.1:3456", loggertesthelper.Logger(), "networkReader", &writer)

			loggertesthelper.TestLoggerSink.Clear()
			go reader.Start()

			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening on port 127.0.0.1:3456"))
		})

		AfterEach(func() {
			reader.Stop()
		})

		It("sends data recieved on UDP socket to its writer", func() {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", "localhost:3456")

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
			connection, err := net.Dial("udp", "localhost:3456")
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
