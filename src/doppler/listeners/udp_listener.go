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
	contextName string
	lock        sync.RWMutex
}

func NewUDPListener(host string, batcher Batcher, logger *gosteno.Logger, contextName string) (*UDPListener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	listener := &UDPListener{
		logger:      logger,
		batcher:     batcher,
		host:        host,
		dataChannel: byteChan,
		contextName: contextName,
	}

	return listener, byteChan
}

func (udp *UDPListener) Address() string {
	return udp.connection.LocalAddr().String()
}

func (udp *UDPListener) Start() {
	connection, err := net.ListenPacket("udp", udp.host)
	if err != nil {
		udp.logger.Fatalf("Failed to listen on port. %s", err)
	}

	udp.logger.Infof("UDP listener listening on port %s", udp.host)
	udp.lock.Lock()
	udp.connection = connection
	udp.lock.Unlock()

	messageCountMetricName := udp.contextName + ".receivedMessageCount"
	listenerTotalMetricName := "listeners.totalReceivedMessageCount"
	receivedByteCountMetricName := udp.contextName + ".receivedByteCount"
	// TODO: will be deprecated
	dropsondeMessageCountMetricName := "dropsondeListener.receivedMessageCount"
	dropsondeReceivedByteCountMetricName := "dropsondeListener.receivedByteCount"

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(udp.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			udp.logger.Debugf("Error while reading: %s", err)
			return
		}
		udp.logger.Debugf("AgentListener: Read %d bytes from address %s", readCount, senderAddr)

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		//TODO: will be deprecated
		udp.batcher.BatchIncrementCounter(dropsondeMessageCountMetricName)
		udp.batcher.BatchAddCounter(dropsondeReceivedByteCountMetricName, uint64(readCount))

		udp.batcher.BatchIncrementCounter(messageCountMetricName)
		udp.batcher.BatchIncrementCounter(listenerTotalMetricName)
		udp.batcher.BatchAddCounter(receivedByteCountMetricName, uint64(readCount))

		udp.dataChannel <- readData
	}
}

func (udp *UDPListener) Stop() {
	udp.lock.Lock()
	defer udp.lock.Unlock()
	udp.connection.Close()
}
