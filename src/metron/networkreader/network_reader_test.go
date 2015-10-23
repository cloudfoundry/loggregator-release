package networkreader_test

import (
	"fmt"
	"net"

	"metron/networkreader"
	"metron/writers/mocks"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"strconv"
	"time"
)

func randomPort() int {
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:1")
	Expect(err).NotTo(HaveOccurred())
	conn, err := net.DialUDP("udp", nil, addr)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	_, addrPort, err := net.SplitHostPort(conn.LocalAddr().String())
	Expect(err).NotTo(HaveOccurred())
	port, err := strconv.Atoi(addrPort)
	Expect(err).NotTo(HaveOccurred())
	return port
}

var _ = Describe("NetworkReader", func() {
	var reader *networkreader.NetworkReader
	var readerStopped chan struct{}
	var writer mocks.MockByteArrayWriter
	var port int
	var address string
	var fakeMetricSender *fake.FakeMetricSender

	BeforeEach(func() {
		loggertesthelper.TestLoggerSink.Clear()

		port = randomPort() + GinkgoParallelNode()
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		writer = mocks.MockByteArrayWriter{}
		var err error
		reader, err = networkreader.New(address, "networkReader", &writer, loggertesthelper.Logger())
		Expect(err).NotTo(HaveOccurred())
		readerStopped = make(chan struct{})
	})

	Context("with a reader running", func() {
		BeforeEach(func() {
			fakeMetricSender = fake.NewFakeMetricSender()
			metricBatcher := metricbatcher.New(fakeMetricSender, time.Millisecond)
			metrics.Initialize(fakeMetricSender, metricBatcher)

			go func() {
				reader.Start()
				close(readerStopped)
			}()

			expectedLog := fmt.Sprintf("Listening on %s", address)
			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring(expectedLog))
		})

		AfterEach(func() {
			reader.Stop()
			<-readerStopped
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

			Eventually(func() uint64 { return fakeMetricSender.GetCounter("networkReader.receivedMessageCount") }).Should(BeEquivalentTo(2))
			Eventually(func() uint64 { return fakeMetricSender.GetCounter("networkReader.receivedByteCount") }).Should(BeEquivalentTo(dataByteCount))

			close(done)
		}, 2)
	})
})
