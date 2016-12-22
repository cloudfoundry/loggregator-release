package listeners

import (
	"crypto/tls"
	"diodes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"

	"doppler/config"
	"plumbing"
)

type TCPListener struct {
	envelopesBuffer *diodes.ManyToOneEnvelope
	batcher         Batcher
	listener        net.Listener
	protocol        string
	connections     map[net.Conn]struct{}
	unmarshaller    *dropsonde_unmarshaller.DropsondeUnmarshaller
	lock            sync.Mutex
	listenerClosed  chan struct{}
	started         bool
	metricProto     string
	deadline        time.Duration
}

func NewTCPListener(
	metricProto, address string,
	tlsListenerConfig *config.TLSListenerConfig,
	envelopesBuffer *diodes.ManyToOneEnvelope,
	batcher Batcher,
	deadline time.Duration,
) (*TCPListener, error) {
	protocol := "TCP"
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	if tlsListenerConfig != nil {
		protocol = "TLS"
		tlsConfig, err := plumbing.NewMutualTLSConfig(tlsListenerConfig.CertFile, tlsListenerConfig.KeyFile, tlsListenerConfig.CAFile, "")
		if err != nil {
			return nil, err
		}
		listener = tls.NewListener(listener, tlsConfig)
	}

	return &TCPListener{
		listener:        listener,
		protocol:        protocol,
		envelopesBuffer: envelopesBuffer,
		batcher:         batcher,
		metricProto:     metricProto,
		deadline:        deadline,

		connections:    make(map[net.Conn]struct{}),
		unmarshaller:   dropsonde_unmarshaller.NewDropsondeUnmarshaller(),
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
		log.Fatalf("%s listener has already been started", t.protocol)
	}
	t.started = true
	listener := t.listener
	t.lock.Unlock()

	log.Printf("%s listener listening on %s", t.protocol, t.Address())
	for {
		conn, err := listener.Accept()
		if err != nil {
			close(t.listenerClosed)
			log.Printf("Error while reading: %s", err)
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

	t.listener.Close()
	t.listener = nil

	for conn := range t.connections {
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
			log.Printf("TLS handshake error on %s: %s", conn.RemoteAddr(), err)
			return
		}
	}

	var (
		n     uint32
		bytes []byte
		err   error
	)

	for {
		conn.SetDeadline(time.Now().Add(t.deadline))
		err = binary.Read(conn, binary.LittleEndian, &n)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error while decoding: %v", err)
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
			log.Printf("Error during i/o read: %v", err)
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

		t.envelopesBuffer.Set(envelope)
	}
}
