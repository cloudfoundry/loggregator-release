package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Statsd support", func() {
	var testServer net.PacketConn

	BeforeEach(func() {
		testServer = eventuallyListensForUDP("localhost:3457")

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/doppler_z1/0",
			Value: []byte("localhost"),
		}

		adapter := etcdRunner.Adapter()
		adapter.Create(node)
		adapter.Disconnect()
		time.Sleep(200 * time.Millisecond) // FIXME: wait for metron to discover the fake doppler ... better ideas welcome

		Eventually(metronSession).Should(gbytes.Say("Listening for statsd on host localhost:51162"))
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("outputs gauges as signed value metric messages", func(done Done) {
		connection, err := net.Dial("udp", "localhost:51162")
		Expect(err).ToNot(HaveOccurred())
		defer connection.Close()

		statsdmsg := []byte("fake-origin.test.gauge:23|g")
		_, err = connection.Write(statsdmsg)
		Expect(err).ToNot(HaveOccurred())

		readBuffer := make([]byte, 65535)
		readCount, _, _ := testServer.ReadFrom(readBuffer)
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])
		readData = readData[32:]

		var receivedEnvelope events.Envelope
		Expect(proto.Unmarshal(readData, &receivedEnvelope)).To(Succeed())

		Expect(receivedEnvelope.GetValueMetric()).To(Equal(basicValueMetric()))
		Expect(receivedEnvelope.GetOrigin()).To(Equal("fake-origin"))
		close(done)
	}, 5)
})
