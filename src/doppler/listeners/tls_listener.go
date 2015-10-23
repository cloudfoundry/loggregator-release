package listeners

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type TLSListener struct {
	envelopeChan   chan *events.Envelope
	logger         *gosteno.Logger
	config         *tls.Config
	listener       net.Listener
	connections    map[net.Conn]struct{}
	unmarshaller   dropsonde_unmarshaller.DropsondeUnmarshaller
	stopped        chan struct{}
	lock           sync.Mutex
	listenerClosed chan struct{}
	started        bool

	messageCountMetricName      string
	receivedByteCountMetricName string
}

func NewTLSListener(contextName string, address string, config *tls.Config, envelopeChan chan *events.Envelope, logger *gosteno.Logger) (Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return &TLSListener{
		listener:       listener,
		envelopeChan:   envelopeChan,
		logger:         logger,
		config:         config,
		connections:    make(map[net.Conn]struct{}),
		unmarshaller:   dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger),
		stopped:        make(chan struct{}),
		listenerClosed: make(chan struct{}),

		messageCountMetricName:      contextName + ".receivedMessageCount",
		receivedByteCountMetricName: contextName + ".receivedByteCount",
	}, nil
}

func (t *TLSListener) Address() string {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return ""
}

func (t *TLSListener) Start() {
	t.lock.Lock()
	if t.started {
		t.lock.Unlock()
		t.logger.Fatal("TLSListener has already been started")
	}
	t.started = true
	listener := t.listener
	t.lock.Unlock()

	t.logger.Infof("TCP listener listening on %s", t.Address())
	for {
		conn, err := listener.Accept()
		if err != nil {
			close(t.listenerClosed)
			t.logger.Debugf("Error while reading: %s", err)
			return
		}
		t.addConnection(conn)
		go t.handleConnection(conn)
	}
}

func (t *TLSListener) Stop() {
	t.lock.Lock()
	if t.listener == nil {
		t.lock.Unlock()
		return
	}

	close(t.stopped)
	t.listener.Close()
	t.listener = nil

	for conn, _ := range t.connections {
		conn.Close()
	}
	done := t.listenerClosed
	t.lock.Unlock()

	<-done
}

func (t *TLSListener) addConnection(conn net.Conn) {
	t.lock.Lock()
	t.connections[conn] = struct{}{}
	t.lock.Unlock()
}

func (t *TLSListener) removeConnection(conn net.Conn) {
	t.lock.Lock()
	delete(t.connections, conn)
	t.lock.Unlock()
}

func (t *TLSListener) handleConnection(conn net.Conn) {
	var n uint16
	var bytes []byte
	var err error

	for {
		err = binary.Read(conn, binary.LittleEndian, &n)
		if err != nil {
			break
		}

		read := bytes
		if cap(bytes) < int(n) {
			bytes = make([]byte, int(n))
		}
		read = bytes[:n]

		_, err = io.ReadFull(conn, read)
		if err != nil {
			break
		}

		envelope, err := t.unmarshaller.UnmarshallMessage(read)
		if err != nil {
			continue
		}

		metrics.BatchIncrementCounter(t.messageCountMetricName)
		metrics.BatchAddCounter(t.receivedByteCountMetricName, uint64(n+2))

		t.logger.Debugf("Received envelope: %#v", envelope)
		select {
		case t.envelopeChan <- envelope:
		case <-t.stopped:
			return
		}
	}

	t.logger.Debugf("Error while decoding: %v", err)
	conn.Close()
	t.removeConnection(conn)
}
