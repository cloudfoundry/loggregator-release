package listeners

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"

	"doppler/config"
	"plumbing"
)

type TCPListener struct {
	envelopeChan   chan *events.Envelope
	logger         *gosteno.Logger
	batcher        Batcher
	listener       net.Listener
	protocol       string
	connections    map[net.Conn]struct{}
	unmarshaller   *dropsonde_unmarshaller.DropsondeUnmarshaller
	stopped        chan struct{}
	lock           sync.Mutex
	listenerClosed chan struct{}
	started        bool
	metricProto    string
	deadline       time.Duration
}

func NewTCPListener(
	metricProto, address string,
	tlsListenerConfig *config.TLSListenerConfig,
	envelopeChan chan *events.Envelope,
	batcher Batcher,
	deadline time.Duration,
	logger *gosteno.Logger,
) (*TCPListener, error) {
	protocol := "TCP"
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	if tlsListenerConfig != nil {
		protocol = "TLS"
		tlsConfig, err := plumbing.NewTLSConfig(tlsListenerConfig.CertFile, tlsListenerConfig.KeyFile, tlsListenerConfig.CAFile, "")
		if err != nil {
			return nil, err
		}
		listener = tls.NewListener(listener, tlsConfig)
	}

	return &TCPListener{
		listener:     listener,
		protocol:     protocol,
		envelopeChan: envelopeChan,
		logger:       logger,
		batcher:      batcher,
		metricProto:  metricProto,
		deadline:     deadline,

		connections:    make(map[net.Conn]struct{}),
		unmarshaller:   dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger),
		stopped:        make(chan struct{}),
		listenerClosed: make(chan struct{}),
	}, nil
}

func (t *TCPListener) Address() string {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return ""
}

func (t *TCPListener) Start() {
	t.lock.Lock()
	if t.started {
		t.lock.Unlock()
		t.logger.Fatalf("%s listener has already been started", t.protocol)
	}
	t.started = true
	listener := t.listener
	t.lock.Unlock()

	t.logger.Infof("%s listener listening on %s", t.protocol, t.Address())
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

func (t *TCPListener) Stop() {
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

func (t *TCPListener) addConnection(conn net.Conn) {
	t.lock.Lock()
	t.connections[conn] = struct{}{}
	t.lock.Unlock()
}

func (t *TCPListener) removeConnection(conn net.Conn) {
	t.lock.Lock()
	delete(t.connections, conn)
	t.lock.Unlock()
}

func (t *TCPListener) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer t.removeConnection(conn)

	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			t.logger.Warnd(map[string]interface{}{
				"error":   err.Error(),
				"address": conn.RemoteAddr().String(),
			}, "TLS handshake error")
			return
		}
	}

	var (
		n     uint32
		bytes []byte
		err   error
	)

	timeOut := time.NewTimer(time.Second)

	for {
		conn.SetDeadline(time.Now().Add(t.deadline))
		err = binary.Read(conn, binary.LittleEndian, &n)
		if err != nil {
			if err != io.EOF {
				t.logger.Errorf("Error while decoding: %v", err)
			}
			break
		}

		read := bytes
		if cap(bytes) < int(n) {
			bytes = make([]byte, int(n))
		}
		read = bytes[:n]

		conn.SetDeadline(time.Now().Add(t.deadline))
		_, err = io.ReadFull(conn, read)
		if err != nil {
			t.logger.Errorf("Error during i/o read: %v", err)
			break
		}

		envelope, err := t.unmarshaller.UnmarshallMessage(read)
		if err != nil {
			continue
		}
		t.batcher.BatchCounter("listeners.receivedEnvelopes").
			SetTag("protocol", strings.ToLower(t.protocol)).
			SetTag("event_type", envelope.EventType.String()).
			Increment()
		t.batcher.BatchIncrementCounter("listeners.totalReceivedMessageCount")
		t.batcher.BatchIncrementCounter(t.metricProto + ".receivedMessageCount")
		t.batcher.BatchAddCounter(t.metricProto+".receivedByteCount", uint64(n+4))
		t.batcher.BatchAddCounter("listeners.totalReceivedByteCount", uint64(n+4))

		select {
		case t.envelopeChan <- envelope:
		case <-t.stopped:
			return
		case <-timeOut.C:
			t.batcher.BatchCounter("listeners.shedEnvelopes").
				SetTag("protocol", strings.ToLower(t.protocol)).
				SetTag("event_type", envelope.EventType.String()).
				Increment()
		}
		timeOut.Reset(time.Second)
	}
}
