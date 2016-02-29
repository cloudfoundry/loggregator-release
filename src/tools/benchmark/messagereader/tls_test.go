package messagereader_test

import (
	"crypto/tls"
	"doppler/listeners"
	"encoding/binary"
	"fmt"
	"net"
	"tools/benchmark/messagereader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("TLSReader", func() {
	var (
		port         int
		reader       *messagereader.TLSReader
		serverConfig *tls.Config
		clientConfig *tls.Config
	)

	BeforeEach(func() {
		port = 3457
		var err error
		serverConfig, err = listeners.NewTLSConfig(
			"fixtures/server.crt",
			"fixtures/server.key",
			"fixtures/loggregator-ca.crt",
		)
		Expect(err).ToNot(HaveOccurred())
		serverConfig.InsecureSkipVerify = true
		clientConfig, err = listeners.NewTLSConfig(
			"fixtures/client.crt",
			"fixtures/client.key",
			"fixtures/loggregator-ca.crt",
		)
		Expect(err).ToNot(HaveOccurred())
		clientConfig.InsecureSkipVerify = true
		reader = messagereader.NewTLS(port, serverConfig)
	})

	AfterEach(func() {
		reader.Close()
	})

	It("should receive message on specified port", func() {
		tlsWriteValueMessage(port, clientConfig)
		tlsWriteValueMessage(port, clientConfig)
		Eventually(reader.Read()).ShouldNot(BeNil())
		Eventually(reader.Read()).ShouldNot(BeNil())
	})
})

func tlsWriteValueMessage(port int, config *tls.Config) {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	Expect(err).ToNot(HaveOccurred())
	tlsClient := tls.Client(conn, config)

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

	err = binary.Write(tlsClient, binary.LittleEndian, uint32(len(messageBytes)))
	Expect(err).ToNot(HaveOccurred())
	_, err = tlsClient.Write(messageBytes)
	Expect(err).ToNot(HaveOccurred())
}
