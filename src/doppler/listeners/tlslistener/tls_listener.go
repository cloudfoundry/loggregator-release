package tlslistener

import (
	"crypto/tls"
	"doppler/listeners/agentlistener"
	"encoding/gob"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"net"
	"sync"
)

type TLSListener struct {
	address      string
	envelopeChan chan *events.Envelope
	logger       *gosteno.Logger
	config       *tls.Config
	listener     net.Listener
	connections  []net.Conn
	wg           sync.WaitGroup
	lock         sync.Mutex
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
	t.listener, err = tls.Listen("tcp", t.address, t.config)
	if err != nil {
		t.logger.Fatalf("Failed to start TCP listener. %s", err)
	}

	t.wg.Add(1)
	defer t.wg.Done()
	t.logger.Infof("TCP listener listening on %s", t.address)
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			t.logger.Debugf("Error while reading: %s", err)
			return
		}
		t.addConnection(conn)
		go t.handleConnection(conn)
	}

}

func (t *TLSListener) Stop() {
	t.lock.Lock()
	defer t.lock.Unlock()
	for _, conn := range t.connections {
		conn.Close()
	}
	t.connections = nil

	if t.listener != nil {
		t.listener.Close()
	}
	t.wg.Wait()
}

func (t *TLSListener) addConnection(conn net.Conn) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.connections = append(t.connections, conn)
}

func (t *TLSListener) handleConnection(conn net.Conn) {
	t.wg.Add(1)
	defer t.wg.Done()
	decoder := gob.NewDecoder(conn)
	for {
		var envelope events.Envelope
		err := decoder.Decode(&envelope)
		if err != nil {
			t.logger.Debugf("Error while decoding: %v", err)
			conn.Close()
			return
		}
		t.envelopeChan <- &envelope
	}
}
