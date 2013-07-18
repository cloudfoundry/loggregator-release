package agentlistener

import (
	"github.com/cloudfoundry/gosteno"
	"net"
)

type agentListener struct {
	*gosteno.Logger
	host string
}

func NewAgentListener(host string, givenLogger *gosteno.Logger) *agentListener {
	return &agentListener{givenLogger, host}
}

func (agentListener *agentListener) Start() chan []byte {
	dataChannel := make(chan []byte)
	connection, err := net.ListenPacket("udp", agentListener.host)
	agentListener.Infof("Listening on port %s", agentListener.host)
	if err != nil {
		agentListener.Fatalf("Failed to listen on port. %s", err)
		panic(err)
	}
	go func() {
		readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size

		for {
			readCount, senderAddr, err := connection.ReadFrom(readBuffer)
			if err != nil {
				agentListener.Debugf("Error while reading. %s", err)
			}
			agentListener.Debugf("Read %d bytes from address %s", readCount, senderAddr)

			readData := make([]byte, readCount) //pass on buffer in size only of read data
			copy(readData, readBuffer[:readCount])

			dataChannel <- readData
		}
	}()
	return dataChannel
}
