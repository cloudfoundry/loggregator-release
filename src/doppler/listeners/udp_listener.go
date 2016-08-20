package listeners

import (
	"net"
	"sync"

	"github.com/cloudfoundry/gosteno"
)

type UDPListener struct {
	batcher     Batcher
	logger      *gosteno.Logger
	host        string
	dataChannel chan []byte
	connection  net.PacketConn
	metricProto string
	lock        sync.RWMutex
}

func NewUDPListener(host string, batcher Batcher, logger *gosteno.Logger, metricProto string) (*UDPListener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	listener := &UDPListener{
		logger:      logger,
		batcher:     batcher,
		host:        host,
		dataChannel: byteChan,
		metricProto: metricProto,
	}

	return listener, byteChan
}

func (l *UDPListener) Address() string {
	return l.connection.LocalAddr().String()
}

func (l *UDPListener) Start() {
	connection, err := net.ListenPacket("udp", l.host)
	if err != nil {
		l.logger.Fatalf("Failed to listen on port. %s", err)
	}

	l.logger.Infof("UDP listener listening on port %s", l.host)
	l.lock.Lock()
	l.connection = connection
	l.lock.Unlock()

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(l.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			l.logger.Debugf("Error while reading: %s", err)
			return
		}
		l.logger.Debugf("AgentListener: Read %d bytes from address %s", readCount, senderAddr)

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		// TODO: will be deprecated
		l.batcher.BatchIncrementCounter("dropsondeListener.receivedMessageCount")
		l.batcher.BatchAddCounter("dropsondeListener.receivedByteCount", uint64(readCount))

		l.batcher.BatchIncrementCounter(l.metricProto + ".receivedMessageCount")
		l.batcher.BatchIncrementCounter("listeners.totalReceivedMessageCount")
		l.batcher.BatchAddCounter(l.metricProto+".receivedByteCount", uint64(readCount))
		l.batcher.BatchAddCounter("listeners.totalReceivedByteCount", uint64(readCount))

		l.dataChannel <- readData
	}
}

func (l *UDPListener) Stop() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.connection.Close()
}
