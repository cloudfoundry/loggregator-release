package v2

import (
	"errors"
	"math/rand"
	"sync/atomic"
	"unsafe"

	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Conn interface {
	Write(data []*plumbing.Envelope) (err error)
}

type ClientPool struct {
	conns []unsafe.Pointer
}

func New(conns ...Conn) *ClientPool {
	pool := &ClientPool{
		conns: make([]unsafe.Pointer, len(conns)),
	}
	for i := range conns {
		pool.conns[i] = unsafe.Pointer(&conns[i])
	}

	return pool
}

func (c *ClientPool) Write(msgs []*plumbing.Envelope) error {
	seed := rand.Int()
	for i := range c.conns {
		idx := (i + seed) % len(c.conns)
		conn := *(*Conn)(atomic.LoadPointer(&c.conns[idx]))

		if err := conn.Write(msgs); err == nil {
			return nil
		}
	}

	return errors.New("unable to write to any dopplers")
}
