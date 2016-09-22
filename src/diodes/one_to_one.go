package diodes

import (
	"sync/atomic"
	"time"
	"unsafe"
)

type Alerter interface {
	Alert(missed int)
}

type bucket struct {
	data []byte
	seq  uint64
}

type OneToOne struct {
	buffer     []unsafe.Pointer
	writeIndex uint64
	readIndex  uint64
	alerter    Alerter
}

var NewOneToOne = func(size int, alerter Alerter) *OneToOne {
	d := &OneToOne{
		buffer:  make([]unsafe.Pointer, size),
		alerter: alerter,
	}
	d.writeIndex = ^d.writeIndex
	return d
}

func (d *OneToOne) Set(data []byte) {
	writeIndex := atomic.AddUint64(&d.writeIndex, 1)
	idx := writeIndex % uint64(len(d.buffer))
	newBucket := &bucket{
		data: data,
		seq:  writeIndex,
	}

	atomic.StorePointer(&d.buffer[idx], unsafe.Pointer(newBucket))
}

func (d *OneToOne) TryNext() ([]byte, bool) {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	value, ok := d.tryNext(idx)
	if ok {
		atomic.AddUint64(&d.readIndex, 1)
	}
	return value, ok
}

func (d *OneToOne) Next() []byte {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	result := d.pollBuffer(idx)
	atomic.AddUint64(&d.readIndex, 1)

	return result
}

func (d *OneToOne) tryNext(idx uint64) ([]byte, bool) {
	result := (*bucket)(atomic.SwapPointer(&d.buffer[idx], nil))

	if result == nil {
		return nil, false
	}

	if result.seq > d.readIndex {
		d.alerter.Alert(int(result.seq - d.readIndex))
		atomic.StoreUint64(&d.readIndex, result.seq)
	}

	return result.data, true
}

func (d *OneToOne) pollBuffer(idx uint64) []byte {
	for {
		result, ok := d.tryNext(idx)

		if !ok {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		return result
	}
}
