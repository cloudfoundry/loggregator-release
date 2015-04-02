package integration_test

import (
	"net"
	"time"
	"os/exec"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Statsd support", func() {
	var fakeDoppler net.PacketConn

	BeforeEach(func() {
		fakeDoppler = eventuallyListensForUDP("localhost:3457")

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/doppler_z1/0",
			Value: []byte("localhost"),
		}

		adapter := etcdRunner.Adapter()
		adapter.Create(node)
		adapter.Disconnect()
		time.Sleep(200 * time.Millisecond) // FIXME: wait for metron to discover the fake doppler ... better ideas welcome
	})

	AfterEach(func() {
		fakeDoppler.Close()
	})

	Context("with a fake statsd client", func() {
		It("outputs gauges as signed value metric messages", func(done Done) {
			connection, err := net.Dial("udp", "localhost:51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.gauge:23|g\nfake-origin.sampled.gauge:23|g|@0.2")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			checkValueMetric(fakeDoppler, basicValueMetric("test.gauge", 23, "gauge"), "fake-origin")
			checkValueMetric(fakeDoppler, basicValueMetric("sampled.gauge", 115, "gauge"), "fake-origin")


			close(done)
		}, 5)

		It("outputs timings as signed value metric messages with unit 'ms'", func(done Done) {
			connection, err := net.Dial("udp", "localhost:51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.timing:23.5|ms")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			checkValueMetric(fakeDoppler, basicValueMetric("test.timing", 23.5, "ms"), "fake-origin")

			close(done)
		}, 5)

		It("outputs counters as signed value metric messages with unit 'counter'", func(done Done) {
			connection, err := net.Dial("udp", "localhost:51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.counter:42|c")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			checkValueMetric(fakeDoppler, basicValueMetric("test.counter", 42, "counter"), "fake-origin")

			close(done)
		}, 5)
	})

	Context("with a Go statsd client", func() {
		It("forwards gauges as signed value metric messages", func(done Done) {
			clientCommand := exec.Command(pathToGoStatsdClient, "51162")

			clientInput, err := clientCommand.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			clientSession, err := gexec.Start(clientCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			clientInput.Write([]byte("gauge test.gauge 23\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.gauge", 23, "gauge"), "testNamespace")

			clientInput.Write([]byte("gaugedelta test.gauge 7\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.gauge", 30, "gauge"), "testNamespace")

			clientInput.Write([]byte("gaugedelta test.gauge -5\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.gauge", 25, "gauge"), "testNamespace")

			clientInput.Write([]byte("gauge test.gauge 50\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.gauge", 50, "gauge"), "testNamespace")

			clientInput.Close()
			clientSession.Kill().Wait()
			close(done)
		}, 5)

		It("forwards timings as signed value metric messages", func(done Done) {
			clientCommand := exec.Command(pathToGoStatsdClient, "51162")

			clientInput, err := clientCommand.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			clientSession, err := gexec.Start(clientCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			clientInput.Write([]byte("timing test.timing 23\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.timing", 23, "ms"), "testNamespace")

			clientInput.Close()
			clientSession.Kill().Wait()
			close(done)
		}, 5)

		It("forwards counters as signed value metric messages", func(done Done) {
			clientCommand := exec.Command(pathToGoStatsdClient, "51162")

			clientInput, err := clientCommand.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			clientSession, err := gexec.Start(clientCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			clientInput.Write([]byte("count test.counter 27\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.counter", 27, "counter"), "testNamespace")

			clientInput.Write([]byte("count test.counter +3\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.counter", 30, "counter"), "testNamespace")

			clientInput.Write([]byte("count test.counter -10\n"))
			Eventually(metronSession).Should(gbytes.Say("StatsdListener: Read "))

			checkValueMetric(fakeDoppler, basicValueMetric("test.counter", 20, "counter"), "testNamespace")

			clientInput.Close()
			clientSession.Kill().Wait()
			close(done)
		}, 5)
	})
})

func checkValueMetric(fakeDoppler net.PacketConn, valueMetric *events.ValueMetric, origin string) {
	readBuffer := make([]byte, 65535)
	readCount, _, _ := fakeDoppler.ReadFrom(readBuffer)
	readData := make([]byte, readCount)
	copy(readData, readBuffer[:readCount])
	Expect(len(readData)).To(BeNumerically(">", 32), "Failed to read enough data to be a message")
	readData = readData[32:]

	var receivedEnvelope events.Envelope
	Expect(proto.Unmarshal(readData, &receivedEnvelope)).To(Succeed())

	Expect(receivedEnvelope.GetValueMetric()).To(Equal(valueMetric))
	Expect(receivedEnvelope.GetOrigin()).To(Equal(origin))
}
