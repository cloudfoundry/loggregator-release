package networkreader_test

import (
	"fmt"
	"metron/networkreader"
	"metron/writers/mocks"
	"net"
	"strconv"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
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
	var (
		reader           *networkreader.NetworkReader
		readerStopped    chan struct{}
		writer           mocks.MockByteArrayWriter
		port             int
		address          string
		fakeMetricSender *fake.FakeMetricSender
	)

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
		var mockBatcher *mockMetricBatcher

		BeforeEach(func() {
			fakeMetricSender = fake.NewFakeMetricSender()
			mockBatcher = newMockMetricBatcher()
			metrics.Initialize(fakeMetricSender, mockBatcher)

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

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			Eventually(writer.Data).Should(HaveLen(2))

			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("networkReader.receivedMessageCount"),
			))
			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("networkReader.receivedByteCount", uint64(len(expectedData))),
				With("networkReader.receivedByteCount", uint64(len(otherData))),
			))

			close(done)
		}, 2)
	})
})
