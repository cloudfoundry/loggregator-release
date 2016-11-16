package legacyclientpool

import (
	"doppler/dopplerservice"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
)

type Finder interface {
	Next() dopplerservice.Event
}

type ClientPool struct {
	mu       sync.RWMutex
	dopplers []*net.UDPConn

	finder Finder
}

func New(finder Finder) *ClientPool {
	p := &ClientPool{
		finder: finder,
	}
	go p.maintainDopplers()

	return p
}

func (p *ClientPool) Write(data []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.dopplers) == 0 {
		return fmt.Errorf("unable to find a Doppler")
	}

	conn := p.dopplers[rand.Intn(len(p.dopplers))]

	_, err := conn.Write(data)
	if err != nil {
		log.Printf("failed to write to Doppler: %s", err)
		return err
	}

	return nil
}

func (p *ClientPool) maintainDopplers() {
	for {
		event := p.finder.Next()
		log.Printf("UDPPool: New event from finder: %v", event.UDPDopplers)
		p.createConns(event.UDPDopplers)
	}
}

func (p *ClientPool) createConns(addrs []string) {
	var conns []*net.UDPConn
	for _, addr := range addrs {
		raddr, err := net.ResolveUDPAddr("udp4", addr)
		if err != nil {
			log.Printf("unable to resolve Doppler addr (%s): %s", addr, err)
			continue
		}

		conn, err := net.DialUDP("udp4", nil, raddr)
		if err != nil {
			log.Printf("unable to DialUDP Doppler (%s): %s", addr, err)
			continue
		}

		conns = append(conns, conn)
	}

	p.mu.Lock()
	for _, conn := range p.dopplers {
		conn.Close()
	}

	defer p.mu.Unlock()
	p.dopplers = conns
}
