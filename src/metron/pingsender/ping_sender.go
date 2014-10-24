package pingsender

import (
	"net"
	"sync"
	"time"
)

type PingSender struct {
	pingTargets  map[string]pingTarget
	pingInterval time.Duration
	pingTimeout  time.Duration
	sync.Mutex
}

func NewPingSender(interval time.Duration) *PingSender {
	return &PingSender{
		pingTargets:  make(map[string]pingTarget),
		pingInterval: interval,
		pingTimeout:  interval * 5,
	}
}

type pingTarget struct {
	stopChan        chan (struct{})
	deregisterTimer *time.Timer
}

func (pinger *PingSender) StartPing(senderAddr net.Addr, connection net.PacketConn) {
	if pinger.senderKnown(senderAddr) {
		pinger.resetTimer(senderAddr)
		return
	}

	pinger.register(senderAddr)
	pinger.sendPings(senderAddr, connection)
}

func (pinger *PingSender) StopPing(target net.Addr) {
	pinger.Lock()
	defer pinger.Unlock()

	pTarget, ok := pinger.pingTargets[target.String()]
	if ok {
		close(pTarget.stopChan)
	}
}

func (pinger *PingSender) register(senderAddr net.Addr) {
	pinger.Lock()
	defer pinger.Unlock()

	pTarget := pingTarget{
		stopChan:        make(chan struct{}),
		deregisterTimer: time.AfterFunc(pinger.pingTimeout, func() { pinger.StopPing(senderAddr) }),
	}
	pinger.pingTargets[senderAddr.String()] = pTarget
}

func (pinger *PingSender) sendPings(senderAddr net.Addr, connection net.PacketConn) {
	intervalTicker := time.NewTicker(pinger.pingInterval)

	pinger.Lock()
	pTarget, ok := pinger.pingTargets[senderAddr.String()]
	pinger.Unlock()

	if ok {
		for {
			select {
			case <-pTarget.stopChan:
				intervalTicker.Stop()
				pinger.Lock()
				delete(pinger.pingTargets, senderAddr.String())
				pinger.Unlock()
				return
			case <-intervalTicker.C:
				connection.WriteTo([]byte("Ping"), senderAddr)
			}
		}
	}
}

func (pinger *PingSender) resetTimer(senderAddr net.Addr) {
	pinger.Lock()
	pTarget, ok := pinger.pingTargets[senderAddr.String()]
	pinger.Unlock()

	if ok {
		pTarget.deregisterTimer.Reset(pinger.pingTimeout)
	}
}

func (pinger *PingSender) senderKnown(senderAddr net.Addr) bool {
	pinger.Lock()
	_, ok := pinger.pingTargets[senderAddr.String()]
	pinger.Unlock()

	return ok
}
