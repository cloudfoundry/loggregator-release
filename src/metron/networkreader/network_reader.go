package networkreader

import (
	"net"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type NetworkReader struct {
	host       string
	connection net.PacketConn
	writer     writers.ByteArrayWriter

	contextName string

	logger *gosteno.Logger
}

func New(host string, name string, writer writers.ByteArrayWriter, logger *gosteno.Logger) *NetworkReader {
	return &NetworkReader{
		host:        host,
		contextName: name,
		writer:      writer,
		logger:      logger,
	}
}

func (nr *NetworkReader) Start() {
	connection, err := net.ListenPacket("udp4", nr.host)
	if err != nil {
		nr.logger.Fatalf("Failed to listen on port. %s", err)
	}
	nr.logger.Infof("Listening on port %s", nr.host)
	nr.connection = connection

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			nr.logger.Debugf("Error while reading. %s", err)
			return
		}
		nr.logger.Debugf("NetworkReader: Read %d bytes from address %s", readCount, senderAddr)
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(nr.contextName + ".receivedMessageCount")
		metrics.BatchAddCounter(nr.contextName+".receivedByteCount", uint64(readCount))
		nr.writer.Write(readData)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
