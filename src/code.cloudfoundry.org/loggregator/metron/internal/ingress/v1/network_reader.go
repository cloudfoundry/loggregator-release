package v1

import (
	"log"
	"net"

	"code.cloudfoundry.org/loggregator/diodes"

	gendiodes "github.com/cloudfoundry/diodes"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type ByteArrayWriter interface {
	Write(message []byte)
}

type NetworkReader struct {
	connection net.PacketConn
	writer     ByteArrayWriter

	contextName string
	buffer      *diodes.OneToOne
}

func New(address string, name string, writer ByteArrayWriter) (*NetworkReader, error) {
	connection, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on %s", address)

	return &NetworkReader{
		connection:  connection,
		contextName: name,
		writer:      writer,
		buffer: diodes.NewOneToOne(10000, gendiodes.AlertFunc(func(missed int) {
			log.Printf("network reader dropped messages %d", missed)
			// metric-documentation-v1: (udp.receiveErrorCount) Number of dropped messages
			// inbound to Metron over the v1 (UDP) API
			metrics.BatchAddCounter("udp.receiveErrorCount", uint64(missed))
		})),
	}, nil
}

func (nr *NetworkReader) StartReading() {
	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, _, err := nr.connection.ReadFrom(readBuffer)
		if err != nil {
			log.Printf("Error while reading: %s", err)
			return
		}
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])

		nr.buffer.Set(readData)
	}
}

func (nr *NetworkReader) StartWriting() {
	receivedMessageCountName := nr.contextName + ".receivedMessageCount"
	receivedByteCountName := nr.contextName + ".receivedByteCount"

	for {
		data := nr.buffer.Next()
		// metric-documentation-v1: (dropsondeAgentListener.receivedMessageCount) Number of
		// received messages inbound to Metron over the v1 (UDP) API
		metrics.BatchIncrementCounter(receivedMessageCountName)
		// metric-documentation-v1: (dropsondeAgentListener.receivedByteCount) Number of
		// received bytes inbound to Metron over the v1 (UDP) API
		metrics.BatchAddCounter(receivedByteCountName, uint64(len(data)))
		nr.writer.Write(data)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
