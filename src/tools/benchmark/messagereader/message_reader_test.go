package messagereader_test

import (
	"fmt"
	"net"
	"tools/benchmark/messagereader"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UdpMessageReader", func() {
	var port int
	var reader *messagereader.MessageReader

	BeforeEach(func() {
		port = 3457
		reader = messagereader.New(port)
	})

	AfterEach(func() {
		reader.Close()
	})

	It("should receive message on specified port", func() {
		writeValueMessage(port)
		writeValueMessage(port)
		Eventually(reader.Read()).ShouldNot(BeNil())
		Eventually(reader.Read()).ShouldNot(BeNil())
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
