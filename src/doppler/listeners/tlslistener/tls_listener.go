package tlslistener

import (
	"crypto/tls"
	"doppler/listeners/agentlistener"
	"encoding/gob"
	"net"
	"sync"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type TLSListener struct {
	address        string
	envelopeChan   chan *events.Envelope
	logger         *gosteno.Logger
	config         *tls.Config
	listener       net.Listener
	connections    map[net.Conn]struct{}
	stopped        chan struct{}
	lock           sync.Mutex
	listenerClosed chan struct{}
}

func New(address string, config *tls.Config, envelopeChan chan *events.Envelope, logger *gosteno.Logger) agentlistener.Listener {
	return &TLSListener{
		address:      address,
		envelopeChan: envelopeChan,
		logger:       logger,
		config:       config,
	}
}

func (t *TLSListener) Start() {
	var err error
	listenerClosed := make(chan struct{})

	t.lock.Lock()
	t.connections = make(map[net.Conn]struct{})
	t.stopped = make(chan struct{})
	t.listenerClosed = listenerClosed
	t.listener, err = tls.Listen("tcp", t.address, t.config)
	t.lock.Unlock()

	if err != nil {
		t.logger.Fatalf("Failed to start TCP listener. %s", err)
	}

	t.logger.Infof("TCP listener listening on %s", t.address)
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			close(listenerClosed)
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
	decoder := gob.NewDecoder(conn)
	for {
		var envelope events.Envelope
		err := decoder.Decode(&envelope)
		t.logger.Debugf("Received envelope: %#v", envelope)
		if err != nil {
			t.logger.Debugf("Error while decoding: %v", err)
			conn.Close()
			t.removeConnection(conn)
			return
		}
		select {
		case t.envelopeChan <- &envelope:
		case <-t.stopped:
			return
		}
	}
}
