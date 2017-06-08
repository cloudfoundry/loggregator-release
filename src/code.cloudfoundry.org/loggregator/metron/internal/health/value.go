package health

import "sync/atomic"

type Value struct {
	number int64
}

func (v *Value) Increment(delta int64) {
	atomic.AddInt64(&v.number, delta)
}

func (v *Value) Decrement(delta int64) {
	v.Increment(-delta)
}

func (v *Value) Number() int64 {
	return atomic.LoadInt64(&v.number)
}
