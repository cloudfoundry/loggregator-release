package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"net"
)

type AgentListener struct {
	host string
}

var logger *gosteno.Logger

func newAgentListener(host string, givenLogger *gosteno.Logger) (*AgentListener) {
	logger = givenLogger

	return &AgentListener{host}
}

func (agentListener *AgentListener) start() (chan []byte) {
	dataChannel := make(chan []byte)
	connection, err := net.ListenPacket("udp", agentListener.host)
	logger.Infof("Listening on port %s", agentListener.host)
	if err != nil {
		logger.Fatalf("Failed to listen on port. %s", err)
		panic(err)
	}
	go func() {
		for {
			data := make([]byte, 4096)

			readCount, senderAddr, err := connection.ReadFrom(data)
			if err != nil {
				logger.Debugf("Error while reading. %s", err)
			}
			logger.Debugf("Read %d bytes from address %s", readCount, senderAddr)
			dataChannel <- data[:readCount]
		}
	}()
	return dataChannel
}
