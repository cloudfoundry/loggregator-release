package networkreader

import (
	"net"

	"metron/writers"

	"github.com/cloudfoundry/dropsonde/logging"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type NetworkReader struct {
	connection net.PacketConn
	writer     writers.ByteArrayWriter

	contextName string

	logger *gosteno.Logger
}

func New(address string, name string, writer writers.ByteArrayWriter, logger *gosteno.Logger) (*NetworkReader, error) {
	connection, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}
	logger.Infof("Listening on %s", address)

	return &NetworkReader{
		connection:  connection,
		contextName: name,
		writer:      writer,
		logger:      logger,
	}, nil
}

func (nr *NetworkReader) Start() {
	receivedMessageCountName := nr.contextName + ".receivedMessageCount"
	receivedByteCountName := nr.contextName + ".receivedByteCount"

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, senderAddr, err := nr.connection.ReadFrom(readBuffer)
		if err != nil {
			nr.logger.Errorf("Error while reading. %s", err)
			return
		}
		logging.Debugf(nr.logger, "NetworkReader: Read %d bytes from address %s", readCount, senderAddr)
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(receivedMessageCountName)
		metrics.BatchAddCounter(receivedByteCountName, uint64(readCount))
		nr.writer.Write(readData)
	}
}

func (nr *NetworkReader) Stop() {
	nr.connection.Close()
}
