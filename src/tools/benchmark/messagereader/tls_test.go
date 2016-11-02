package messagereader_test

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"plumbing"
	"tools/benchmark/messagereader"
)

var _ = Describe("TLSReader", func() {
	var (
		port         int = 3457
		reader       *messagereader.TLSReader
		serverConfig *tls.Config
		clientConfig *tls.Config
	)

	BeforeEach(func() {
		var err error
		serverConfig, err = plumbing.NewTLSConfig(
			"fixtures/server.crt",
			"fixtures/server.key",
			"fixtures/loggregator-ca.crt",
			"",
		)
		Expect(err).ToNot(HaveOccurred())
		serverConfig.InsecureSkipVerify = true
		clientConfig, err = plumbing.NewTLSConfig(
			"fixtures/client.crt",
			"fixtures/client.key",
			"fixtures/loggregator-ca.crt",
			"",
		)
		Expect(err).ToNot(HaveOccurred())
		clientConfig.InsecureSkipVerify = true
		reader = messagereader.NewTLS(port, serverConfig)
	})

	AfterEach(func() {
		port += 1
		reader.Close()
	})

	It("receives a message on the specified port", func() {
		tlsWriteMessage(port, clientConfig)
		tlsWriteMessage(port, clientConfig)
		Eventually(reader.Read()).ShouldNot(BeNil())
		Eventually(reader.Read()).ShouldNot(BeNil())
	})

	It("doesn't panic when message size is smaller than it's prefixed length", func() {
		tlsWriteMessageInParts(port, clientConfig)
		tlsWriteMessageInParts(port, clientConfig)
		Eventually(reader.Read()).ShouldNot(BeNil())
		Eventually(reader.Read()).ShouldNot(BeNil())
	})
})

func tlsWriteMessage(port int, config *tls.Config) {
	messageBytes, tlsClient := createMessage(port, config)

	err := binary.Write(tlsClient, binary.LittleEndian, uint32(len(messageBytes)))
	Expect(err).ToNot(HaveOccurred())
	_, err = tlsClient.Write(messageBytes)
	Expect(err).ToNot(HaveOccurred())
}

func tlsWriteMessageInParts(port int, config *tls.Config) {
	messageBytes, tlsClient := createMessage(port, config)

	err := binary.Write(tlsClient, binary.LittleEndian, uint32(len(messageBytes)))
	Expect(err).ToNot(HaveOccurred())
	_, err = tlsClient.Write(messageBytes[:len(messageBytes)-4])
	Expect(err).ToNot(HaveOccurred())
	_, err = tlsClient.Write(messageBytes[len(messageBytes)-4:])
	Expect(err).ToNot(HaveOccurred())
}

func createMessage(port int, config *tls.Config) ([]byte, io.Writer) {
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
	return messageBytes, tlsClient
}
