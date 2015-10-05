package agentlistener

import (
	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type Listener interface {
	Start()
	Stop()
}

type agentListener struct {
	*gosteno.Logger
	host        string
	dataChannel chan []byte
	connection  net.PacketConn
	contextName string
	sync.RWMutex
}

func NewAgentListener(host string, givenLogger *gosteno.Logger, name string) (Listener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	listener := &agentListener{
		Logger:      givenLogger,
		host:        host,
		dataChannel: byteChan,
		contextName: name,
	}

	return listener, byteChan
}

func (agentListener *agentListener) Start() {
	connection, err := net.ListenPacket("udp", agentListener.host)
	if err != nil {
		agentListener.Fatalf("Failed to listen on port. %s", err)
	}

	agentListener.Infof("Listening on port %s", agentListener.host)
	agentListener.Lock()
	agentListener.connection = connection
	agentListener.Unlock()

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(agentListener.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			agentListener.Debugf("Error while reading. %s", err)
			return
		}
		agentListener.Debugf("AgentListener: Read %d bytes from address %s", readCount, senderAddr)

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(agentListener.contextName + ".receivedMessageCount")
		metrics.BatchAddCounter(agentListener.contextName+".receivedByteCount", uint64(readCount))

		agentListener.dataChannel <- readData
	}

}

func (agentListener *agentListener) Stop() {
	agentListener.Lock()
	defer agentListener.Unlock()
	agentListener.connection.Close()
}
