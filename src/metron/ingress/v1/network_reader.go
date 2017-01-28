package ingress

import (
	"log"
	"net"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type ByteArrayWriter interface {
	Write(message []byte)
}

type NetworkReader struct {
	connection net.PacketConn
	writer     ByteArrayWriter

	contextName string
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
	}, nil
}

func (nr *NetworkReader) Start() {
	receivedMessageCountName := nr.contextName + ".receivedMessageCount"
	receivedByteCountName := nr.contextName + ".receivedByteCount"

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, _, err := nr.connection.ReadFrom(readBuffer)
		if err != nil {
			log.Printf("Error while reading: %s", err)
			return
		}
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(receivedMessageCountName)
		metrics.BatchAddCounter(receivedByteCountName, uint64(readCount))
		nr.writer.Write(readData)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
