package agentlistener

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"net"
	"sync/atomic"
)

type agentListener struct {
	*gosteno.Logger
	host                 string
	receivedMessageCount *uint64
	receivedByteCount    *uint64
	dataChannel          chan []byte
}

func NewAgentListener(host string, givenLogger *gosteno.Logger) *agentListener {
	return &agentListener{givenLogger, host, new(uint64), new(uint64), make(chan []byte, 1024)}
}

func (agentListener *agentListener) Start() chan []byte {
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

			atomic.AddUint64(agentListener.receivedMessageCount, 1)
			atomic.AddUint64(agentListener.receivedByteCount, uint64(readCount))
			agentListener.dataChannel <- readData
		}
	}()
	return agentListener.dataChannel
}

func (agentListener *agentListener) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "agentListener",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "currentBufferCount", Value: len(agentListener.dataChannel)},
			instrumentation.Metric{Name: "receivedMessageCount", Value: atomic.LoadUint64(agentListener.receivedMessageCount)},
			instrumentation.Metric{Name: "receivedByteCount", Value: atomic.LoadUint64(agentListener.receivedByteCount)},
		},
	}
}
