package clientpool

import (
	"errors"
	"math/rand"
	"sync/atomic"
	"unsafe"

	v2 "plumbing/v2"
)

type Conn interface {
	Write(data *v2.Envelope) (err error)
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

func (c *ClientPool) Write(msg *v2.Envelope) error {
	seed := rand.Int()
	for i := range c.conns {
		idx := (i + seed) % len(c.conns)
		conn := *(*Conn)(atomic.LoadPointer(&c.conns[idx]))

		if err := conn.Write(msg); err == nil {
			return nil
		}
	}

	return errors.New("unable to write to any dopplers")
}
