package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"net"
)

type agentListener struct {
	host string
}

var logger *gosteno.Logger

func NewAgentListener(host string, givenLogger *gosteno.Logger) (*agentListener) {
	logger = givenLogger

	return &agentListener{host}
}

func (agentListener *agentListener) Start() (chan []byte) {
	dataChannel := make(chan []byte)
	connection, err := net.ListenPacket("udp", agentListener.host)
	logger.Infof("Listening on port %s", agentListener.host)
	if err != nil {
		logger.Fatalf("Failed to listen on port. %s", err)
		panic(err)
	}
	go func() {
		readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size

		for {
			readCount, senderAddr, err := connection.ReadFrom(readBuffer)
			if err != nil {
				logger.Debugf("Error while reading. %s", err)
			}
			logger.Debugf("Read %d bytes from address %s", readCount, senderAddr)

			readData := make([]byte, readCount) //pass on buffer in size only of read data
			copy(readData, readBuffer[:readCount])

			dataChannel <- readData
		}
	}()
	return dataChannel
}
