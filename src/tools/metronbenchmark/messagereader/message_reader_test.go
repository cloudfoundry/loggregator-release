package messagereader_test

import (
	"fmt"
	"net"
	"tools/metronbenchmark/messagereader"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UdpMessageReader", func() {
	var port int
	var reporter *fakeReporter
	var reader *messagereader.MessageReader

	BeforeEach(func() {
		port = 3457
		reporter = &fakeReporter{}
		reader = messagereader.NewMessageReader(port, reporter)
	})

	AfterEach(func() {
		reader.Close()
	})

	It("should receive message on specified port", func() {
		writeValueMessage(port)
		writeValueMessage(port)
		reader.Read()
		reader.Read()
		Eventually(func() int { return reporter.totalReceived }).Should(Equal(2))
	})

	It("should not increment total received messages", func() {
		Consistently(func() int { return reporter.totalReceived }).Should(Equal(0))
	})
})

func writeValueMessage(port int) {
	conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", port))
	Expect(err).ToNot(HaveOccurred())

	message := &events.Envelope{
		EventType: events.Envelope_ValueMetric.Enum(),
		Origin:    proto.String("someorigin"),

		ValueMetric: &events.ValueMetric{
			Name:  proto.String("some name"),
			Value: proto.Float64(24.0),
			Unit:  proto.String("some unit"),
		},
	}

	messageBytes, err := proto.Marshal(message)
	Expect(err).ToNot(HaveOccurred())

	// Pad the first 32 bytes of the payload with zeroes
	// In reality this would be the signature

	padding := make([]byte, 32)

	payload := append(padding, messageBytes...)
	conn.Write(payload)
}

type fakeReporter struct {
	totalReceived int
}

func (f *fakeReporter) IncrementReceivedMessages() {
	f.totalReceived++
}
