package listeners

import (
	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type Listener interface {
	Address() string
	Start()
	Stop()
}

type udpListener struct {
	*gosteno.Logger
	host        string
	dataChannel chan []byte
	connection  net.PacketConn
	contextName string
	sync.RWMutex
}

func NewUDPListener(host string, givenLogger *gosteno.Logger, name string) (Listener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	listener := &udpListener{
		Logger:      givenLogger,
		host:        host,
		dataChannel: byteChan,
		contextName: name,
	}

	return listener, byteChan
}

func (udp *udpListener) Address() string {
	return udp.connection.LocalAddr().String()
}

func (udp *udpListener) Start() {
	connection, err := net.ListenPacket("udp", udp.host)
	if err != nil {
		udp.Fatalf("Failed to listen on port. %s", err)
	}

	udp.Infof("Listening on port %s", udp.host)
	udp.Lock()
	udp.connection = connection
	udp.Unlock()

	messageCountMetricName := udp.contextName + ".receivedMessageCount"
	receivedByteCountMetricName := udp.contextName + ".receivedByteCount"

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(udp.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			udp.Debugf("Error while reading: %s", err)
			return
		}
		udp.Debugf("AgentListener: Read %d bytes from address %s", readCount, senderAddr)

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(messageCountMetricName)
		metrics.BatchAddCounter(receivedByteCountMetricName, uint64(readCount))

		udp.dataChannel <- readData
	}
}

func (udpListener *udpListener) Stop() {
	udpListener.Lock()
	defer udpListener.Unlock()
	udpListener.connection.Close()
}
