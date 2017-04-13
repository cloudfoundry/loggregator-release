package legacy

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type ClientPool struct {
	mu            sync.Mutex
	conn          *net.UDPConn
	writeCount    int
	maxWriteCount int

	refreshInterval time.Duration
	lastRefreshed   time.Time

	dopplerAddr string
}

func New(dopplerAddr string, maxWriteCount int, ri time.Duration) *ClientPool {
	p := &ClientPool{
		dopplerAddr:     dopplerAddr,
		maxWriteCount:   maxWriteCount,
		refreshInterval: ri,
		lastRefreshed:   time.Now(),
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
		return err
	}

	return nil
}

func (p *ClientPool) fetchConn() *net.UDPConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil && p.writeCount < p.maxWriteCount &&
		time.Since(p.lastRefreshed) < p.refreshInterval {
		p.writeCount++
		return p.conn
	}
	p.writeCount = 0
	p.lastRefreshed = time.Now()

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
