package listeners

import (
	"log"
	"net"
	"sync"
)

type UDPListener struct {
	batcher     Batcher
	host        string
	dataChannel chan []byte
	connection  net.PacketConn
	metricProto string
	lock        sync.RWMutex
}

func NewUDPListener(host string, batcher Batcher, metricProto string) (*UDPListener, <-chan []byte) {
	byteChan := make(chan []byte, 1024)
	listener := &UDPListener{
		batcher:     batcher,
		host:        host,
		dataChannel: byteChan,
		metricProto: metricProto,
	}

	return listener, byteChan
}

func (l *UDPListener) Address() string {
	return l.connection.LocalAddr().String()
}

func (l *UDPListener) Start() {
	connection, err := net.ListenPacket("udp", l.host)
	if err != nil {
		log.Fatalf("Failed to listen on port. %s", err)
	}

	log.Printf("UDP listener listening on port %s", l.host)
	l.lock.Lock()
	l.connection = connection
	l.lock.Unlock()

	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	defer close(l.dataChannel)
	for {
		readCount, _, err := connection.ReadFrom(readBuffer)
		if err != nil {
			log.Printf("error while reading UDP: %s", err)
			return
		}

		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])

		// metric-documentation-v1: (dropsondeListener.receivedMessageCount) DEPRECATED Number of
		// received messages from Metron on the UDP listener. This is a
		// duplicate of udpListener.receivedMessageCount.
		l.batcher.BatchIncrementCounter("dropsondeListener.receivedMessageCount")
		// metric-documentation-v1: (dropsondeListener.receivedByteCount) DEPRECATED Number
		// of bytes received from Metron on the UDP listener. This metric is a
		// duplicate of udpListener.receivedByteCount.
		l.batcher.BatchAddCounter("dropsondeListener.receivedByteCount", uint64(readCount))

		// metric-documentation-v1: (udpListener.receivedByteCount) Number of received
		// messages from Metron on the UDP listener
		l.batcher.BatchIncrementCounter(l.metricProto + ".receivedMessageCount")
		// metric-documentation-v1: (listeners.totalReceivedMessageCount) Total
		// number of messages received by doppler.
		l.batcher.BatchIncrementCounter("listeners.totalReceivedMessageCount")
		// metric-documentation-v1: (udpListener.receivedByteCount) Number of bytes received
		// from Metron on the UDP listener
		l.batcher.BatchAddCounter(l.metricProto+".receivedByteCount", uint64(readCount))
		// metric-documentation-v1: (listeners.totalReceivedByteCount) USELESS Number of bytes
		// received from Metron used only by the UDP listener
		l.batcher.BatchAddCounter("listeners.totalReceivedByteCount", uint64(readCount))

		l.dataChannel <- readData
	}
}

func (l *UDPListener) Stop() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.connection.Close()
}
