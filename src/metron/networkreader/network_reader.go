package networkreader

import (
	"log"
	"net"

	"diodes"
	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type NetworkReader struct {
	connection net.PacketConn
	writer     writers.ByteArrayWriter

	contextName string
	buffer      *diodes.OneToOne
}

func New(address string, name string, writer writers.ByteArrayWriter) (*NetworkReader, error) {
	connection, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on %s", address)

	return &NetworkReader{
		connection:  connection,
		contextName: name,
		writer:      writer,
		buffer: diodes.NewOneToOne(10000, diodes.AlertFunc(func(missed int) {
			log.Printf("network reader dropped messages %d", missed)
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
		metrics.BatchIncrementCounter(receivedMessageCountName)
		metrics.BatchAddCounter(receivedByteCountName, uint64(len(data)))
		nr.writer.Write(data)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
