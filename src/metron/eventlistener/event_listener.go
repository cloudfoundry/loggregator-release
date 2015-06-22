package eventlistener

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type EventListener struct {
	host        string
	dataChannel chan []byte
	connection  net.PacketConn

	receivedMessageCount uint64
	receivedByteCount    uint64
	contextName          string

	lock   sync.RWMutex
	logger *gosteno.Logger
}

func New(host string, name string, logger *gosteno.Logger) (*EventListener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	return &EventListener{host: host, dataChannel: byteChan, contextName: name, logger: logger}, byteChan
}

func (eventListener *EventListener) Start() {
	connection, err := net.ListenPacket("udp", eventListener.host)
	if err != nil {
		eventListener.logger.Fatalf("Failed to listen on port. %s", err)
	}
	eventListener.logger.Infof("Listening on port %s", eventListener.host)
	eventListener.lock.Lock()
	eventListener.connection = connection
	eventListener.lock.Unlock()

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(eventListener.dataChannel)
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			eventListener.logger.Debugf("Error while reading. %s", err)
			return
		}
		eventListener.logger.Debugf("EventListener: Read %d bytes from address %s", readCount, senderAddr)
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		atomic.AddUint64(&eventListener.receivedMessageCount, 1)
		atomic.AddUint64(&eventListener.receivedByteCount, uint64(readCount))
		eventListener.dataChannel <- readData
	}
}

func (eventListener *EventListener) Stop() {
	eventListener.lock.Lock()
	defer eventListener.lock.Unlock()
	eventListener.connection.Close()
}

func (eventListener *EventListener) Emit() instrumentation.Context {
	return instrumentation.Context{Name: eventListener.contextName,
		Metrics: eventListener.metrics(),
	}
}

func (eventListener *EventListener) metrics() []instrumentation.Metric {
	return []instrumentation.Metric{
		instrumentation.Metric{Name: "currentBufferCount", Value: len(eventListener.dataChannel)},
		instrumentation.Metric{Name: "receivedMessageCount", Value: atomic.LoadUint64(&eventListener.receivedMessageCount)},
		instrumentation.Metric{Name: "receivedByteCount", Value: atomic.LoadUint64(&eventListener.receivedByteCount)},
	}
}
