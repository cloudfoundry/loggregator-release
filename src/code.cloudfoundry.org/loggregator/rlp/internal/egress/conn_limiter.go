package egress

import (
	"net"
	"sync"
	"sync/atomic"
)

// LimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func LimitListener(l net.Listener, n int) net.Listener {
	listener := &limitListener{
		max: int32(n),
	}
	listener.Listener = l
	return listener
}

type limitListener struct {
	net.Listener
	max   int32
	count int32
}

func (l *limitListener) tryAcquire() bool {
	if atomic.AddInt32(&l.count, 1) > l.max {
		l.release()
		return false
	}
	return true
}

func (l *limitListener) release() {
	atomic.AddInt32(&l.count, -1)
}

func (l *limitListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		if !l.tryAcquire() {
			c.Close()
			continue
		}

		return &limitListenerConn{Conn: c, release: l.release}, nil
	}
}

type limitListenerConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (l *limitListenerConn) Close() error {
	err := l.Conn.Close()
	l.releaseOnce.Do(l.release)
	return err
}
