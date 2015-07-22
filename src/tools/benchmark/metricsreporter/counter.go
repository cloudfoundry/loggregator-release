package metricsreporter

import (
	"sync"
	"sync/atomic"
)

type Counter struct {
	value uint64
	total uint64
	lock  sync.Mutex
}

func NewCounter() *Counter {
	return &Counter{}
}

func (c *Counter) GetValue() uint64 {
	return atomic.LoadUint64(&c.value)
}

func (c *Counter) GetTotal() uint64 {
	return atomic.LoadUint64(&c.total)
}

func (c *Counter) IncrementValue() {
	atomic.AddUint64(&c.value, 1)
}

func (c *Counter) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()

	atomic.AddUint64(&c.total, atomic.LoadUint64(&c.value))
	atomic.StoreUint64(&c.value, 0)
}
