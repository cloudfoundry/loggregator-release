package legacyclientpool

import (
	"fmt"
	"log"
	"net"
	"sync"
)

type ClientPool struct {
	mu            sync.Mutex
	conn          *net.UDPConn
	writeCount    int
	maxWriteCount int

	dopplerAddr string
}

func New(dopplerAddr string, maxWriteCount int) *ClientPool {
	p := &ClientPool{
		dopplerAddr:   dopplerAddr,
		maxWriteCount: maxWriteCount,
	}

	return p
}

func (p *ClientPool) Write(data []byte) error {
	conn := p.fetchConn()

	if conn == nil {
		return fmt.Errorf("doppler not available (%s)", p.dopplerAddr)
	}

	_, err := conn.Write(data)
	if err != nil {
		log.Printf("failed to write to Doppler: %s", err)
		return err
	}

	return nil
}

func (p *ClientPool) fetchConn() *net.UDPConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil && p.writeCount < p.maxWriteCount {
		p.writeCount++
		return p.conn
	}
	p.writeCount = 0

	raddr, err := net.ResolveUDPAddr("udp4", p.dopplerAddr)
	if err != nil {
		log.Printf("unable to resolve Doppler addr (%s): %s", p.dopplerAddr, err)
		return nil
	}

	p.conn, err = net.DialUDP("udp4", nil, raddr)
	if err != nil {
		log.Printf("unable to DialUDP Doppler (%s): %s", p.dopplerAddr, err)
		return nil
	}

	return p.conn
}
