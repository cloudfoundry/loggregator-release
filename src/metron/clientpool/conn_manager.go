package clientpool

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"
	"unsafe"
)

type Finder interface {
	Doppler() (uri string)
}

type ConnManager struct {
	finder    Finder
	conn      unsafe.Pointer
	writes    int64
	maxWrites int64
}

func NewConnManager(maxWrites int64, finder Finder) *ConnManager {
	m := &ConnManager{
		finder:    finder,
		maxWrites: maxWrites,
	}

	go m.maintainConn()
	return m
}

func (m *ConnManager) Write(data []byte) error {
	conn := atomic.LoadPointer(&m.conn)
	if conn == nil || (*net.UDPConn)(conn) == nil {
		return errors.New("no connection to doppler present")
	}

	udpConn := (*net.UDPConn)(conn)
	n, err := udpConn.Write(data)
	if err != nil {
		// TODO: This won't happen for good reasons with UDP
		// gRPC it can, so we will need to ensure this is tested
		// when we move to gRPC
		atomic.StorePointer(&m.conn, nil)
		udpConn.Close()
		return err
	}

	if n != len(data) {
		return fmt.Errorf("expected to write %d bytes, but only wrote %d", len(data), n)
	}

	if atomic.AddInt64(&m.writes, 1) >= m.maxWrites {
		atomic.StorePointer(&m.conn, nil)
		atomic.StoreInt64(&m.writes, 0)
		udpConn.Close()
	}

	return nil
}

func (m *ConnManager) maintainConn() {
	for range time.Tick(50 * time.Millisecond) {
		conn := atomic.LoadPointer(&m.conn)
		if conn != nil && (*net.UDPConn)(conn) != nil {
			continue
		}

		dopplerURI := m.finder.Doppler()
		dopplerAddr, err := net.ResolveUDPAddr("udp4", dopplerURI)
		if err != nil {
			log.Printf("error resolving doppler URI (%s): %s", dopplerURI, err)
			continue
		}

		udpConn, err := net.DialUDP("udp4", nil, dopplerAddr)
		if err != nil {
			log.Printf("error dialing doppler %s: %s", dopplerURI, err)
			continue
		}

		log.Printf("successfully connected to doppler %s", dopplerURI)
		atomic.StorePointer(&m.conn, unsafe.Pointer(udpConn))
	}
}
