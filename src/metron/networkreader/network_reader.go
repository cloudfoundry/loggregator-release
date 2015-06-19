package networkreader

import (
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type NetworkReader struct {
	host        string
	connection  net.PacketConn
	writer 		io.Writer

	receivedMessageCount uint64
	receivedByteCount    uint64
	contextName          string

	lock   sync.RWMutex
	logger *gosteno.Logger
}

func New(host string, givenLogger *gosteno.Logger, name string, writer io.Writer) *NetworkReader {
	return &NetworkReader{
		logger: givenLogger,
		host: host,
		contextName: name,
		writer: writer,
	}
}

func (nr *NetworkReader) Start() {
	connection, err := net.ListenPacket("udp", nr.host)
	if err != nil {
		nr.logger.Fatalf("Failed to listen on port. %s", err)
	}
	nr.logger.Infof("Listening on port %s", nr.host)
	nr.lock.Lock()
	nr.connection = connection
	nr.lock.Unlock()

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	for {
		readCount, senderAddr, err := connection.ReadFrom(readBuffer)
		if err != nil {
			nr.logger.Debugf("Error while reading. %s", err)
			return
		}
		nr.logger.Debugf("NetworkReader: Read %d bytes from address %s", readCount, senderAddr)
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		atomic.AddUint64(&nr.receivedMessageCount, 1)
		atomic.AddUint64(&nr.receivedByteCount, uint64(readCount))
		nr.writer.Write(readData)
	}
}

func (nr *NetworkReader) Stop() {
	nr.lock.Lock()
	defer nr.lock.Unlock()
	nr.connection.Close()
}

func (nr *NetworkReader) Emit() instrumentation.Context {
	return instrumentation.Context{Name: nr.contextName,
		Metrics: nr.metrics(),
	}
}

func (nr *NetworkReader) metrics() []instrumentation.Metric {
	return []instrumentation.Metric{
		instrumentation.Metric{Name: "receivedMessageCount", Value: atomic.LoadUint64(&nr.receivedMessageCount)},
		instrumentation.Metric{Name: "receivedByteCount", Value: atomic.LoadUint64(&nr.receivedByteCount)},
	}
}
