package integration_test

import (
	"io"
	"net"
	"os/exec"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Statsd support", func() {
	var fakeDoppler net.PacketConn
	var getValueMetric = func() *events.ValueMetric {
		readBuffer := make([]byte, 65535)
		readCount, _, _ := fakeDoppler.ReadFrom(readBuffer)
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])
		Expect(len(readData)).To(BeNumerically(">", 32), "Failed to read enough data to be a message")
		readData = readData[32:]

		var receivedEnvelope events.Envelope
		Expect(proto.Unmarshal(readData, &receivedEnvelope)).To(Succeed())
		return receivedEnvelope.ValueMetric
	}

	BeforeEach(func() {
		fakeDoppler = eventuallyListensForUDP("127.0.0.1:3457")

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/doppler_z1/0",
			Value: []byte("127.0.0.1"),
		}

		adapter := etcdRunner.Adapter(nil)
		adapter.Create(node)
		adapter.Disconnect()
	})

	AfterEach(func() {
		fakeDoppler.Close()
	})

	Context("with a fake statsd client", func() {
		It("outputs gauges as signed value metric messages", func() {
			connection, err := net.Dial("udp4", "127.0.0.1:51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.gauge:23|g\nfake-origin.sampled.gauge:23|g|@0.2")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			expected := basicValueMetric("test.gauge", 23, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))

			expected = basicValueMetric("sampled.gauge", 115, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))
		})

		It("outputs timings as signed value metric messages with unit 'ms'", func(done Done) {
			defer close(done)
			connection, err := net.Dial("udp", ":51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.timing:23.5|ms")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			expected := basicValueMetric("test.timing", 23.5, "ms")
			Eventually(getValueMetric).Should(Equal(expected))
		}, 5)

		It("outputs counters as signed value metric messages with unit 'counter'", func(done Done) {
			defer close(done)
			connection, err := net.Dial("udp", ":51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()

			statsdmsg := []byte("fake-origin.test.counter:42|c")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			expected := basicValueMetric("test.counter", 42, "counter")
			Eventually(getValueMetric).Should(Equal(expected))
		}, 5)
	})

	Context("with a Go statsd client", func() {
		var (
			clientInput   io.WriteCloser
			clientSession *gexec.Session
			err           error
		)

		BeforeEach(func() {
			clientCommand := exec.Command(pathToGoStatsdClient, "51162")

			clientInput, err = clientCommand.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			clientSession, err = gexec.Start(clientCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			clientInput.Close()
			clientSession.Kill().Wait()
		})

		It("forwards gauges as signed value metric messages", func(done Done) {
			defer close(done)
			clientInput.Write([]byte("gauge test.gauge 23\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected := basicValueMetric("test.gauge", 23, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))

			clientInput.Write([]byte("gaugedelta test.gauge 7\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected = basicValueMetric("test.gauge", 30, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))

			clientInput.Write([]byte("gaugedelta test.gauge -5\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected = basicValueMetric("test.gauge", 25, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))

			clientInput.Write([]byte("gauge test.gauge 50\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected = basicValueMetric("test.gauge", 50, "gauge")
			Eventually(getValueMetric).Should(Equal(expected))
		}, 5)

		It("forwards timings as signed value metric messages", func(done Done) {
			defer close(done)
			clientInput.Write([]byte("timing test.timing 23\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected := basicValueMetric("test.timing", 23, "ms")
			Eventually(getValueMetric).Should(Equal(expected))
		}, 5)

		It("forwards counters as signed value metric messages", func(done Done) {
			defer close(done)
			clientInput.Write([]byte("count test.counter 27\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected := basicValueMetric("test.counter", 27, "counter")
			Eventually(getValueMetric).Should(Equal(expected))

			clientInput.Write([]byte("count test.counter +3\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected = basicValueMetric("test.counter", 30, "counter")
			Eventually(getValueMetric).Should(Equal(expected))

			clientInput.Write([]byte("count test.counter -10\n"))
			Eventually(statsdInjectorSession).Should(gbytes.Say("StatsdListener: Read "))

			expected = basicValueMetric("test.counter", 20, "counter")
			Eventually(getValueMetric).Should(Equal(expected))
		}, 5)
	})
})
