package listeners

import (
	"crypto/tls"
	"crypto/x509"
	"doppler/config"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"sync"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
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
}

func NewTLSConfig(certFile, keyFile, caCertFile string) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err.Error())
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		MinVersion:         tls.VersionTLS12,
	}

	if caCertFile != "" {
		certBytes, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed read ca cert file: %s", err.Error())
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
			return nil, errors.New("Unable to load caCert")
		}
		tlsConfig.RootCAs = caCertPool
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}

func NewTCPListener(
	metricProto, address string,
	tlsListenerConfig *config.TLSListenerConfig,
	envelopeChan chan *events.Envelope,
	batcher Batcher,
	logger *gosteno.Logger,
) (*TCPListener, error) {
	protocol := "TCP"
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	if tlsListenerConfig != nil {
		protocol = "TLS"
		tlsConfig, err := NewTLSConfig(tlsListenerConfig.CertFile, tlsListenerConfig.KeyFile, tlsListenerConfig.CAFile)
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

	errCountMetric := t.metricProto + ".receiveErrorCount"

	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			t.logger.Warnd(map[string]interface{}{
				"error":   err.Error(),
				"address": conn.RemoteAddr().String(),
			}, "TLS handshake error")
			t.batcher.BatchIncrementCounter(errCountMetric)
			return
		}
	}

	var (
		n     uint32
		bytes []byte
		err   error
	)

	for {
		err = binary.Read(conn, binary.LittleEndian, &n)
		if err != nil {
			if err != io.EOF {
				t.batcher.BatchIncrementCounter(errCountMetric)
				t.logger.Errorf("Error while decoding: %v", err)
			}
			break
		}

		read := bytes
		if cap(bytes) < int(n) {
			bytes = make([]byte, int(n))
		}
		read = bytes[:n]

		_, err = io.ReadFull(conn, read)
		if err != nil {
			t.batcher.BatchIncrementCounter(errCountMetric)
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
		}
	}
}
