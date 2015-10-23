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

func (agentListener *agentListener) Address() string {
	return agentListener.connection.LocalAddr().String()
}

func (agent *agentListener) Start() {
	connection, err := net.ListenPacket("udp", agent.host)
	if err != nil {
		agent.Fatalf("Failed to listen on port. %s", err)
	}

	agent.Infof("Listening on port %s", agent.host)
	agent.Lock()
	agent.connection = connection
	agent.Unlock()

	messageCountMetricName := agent.contextName + ".receivedMessageCount"
	receivedByteCountMetricName := agent.contextName + ".receivedByteCount"

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(agent.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			agent.Debugf("Error while reading. %s", err)
			return
		}
		agent.Debugf("AgentListener: Read %d bytes from address %s", readCount, senderAddr)

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		metrics.BatchIncrementCounter(messageCountMetricName)
		metrics.BatchAddCounter(receivedByteCountMetricName, uint64(readCount))

		agent.dataChannel <- readData
	}
}

func (agentListener *agentListener) Stop() {
	agentListener.Lock()
	defer agentListener.Unlock()
	agentListener.connection.Close()
}
