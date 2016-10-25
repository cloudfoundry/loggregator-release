package diodes

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// ManyToOne diode is optimal for many writers and a single
// reader.
type ManyToOne struct {
	buffer     []unsafe.Pointer
	writeIndex uint64
	readIndex  uint64
	alerter    Alerter
}

var NewManyToOne = func(size int, alerter Alerter) *ManyToOne {
	d := &ManyToOne{
		buffer:  make([]unsafe.Pointer, size),
		alerter: alerter,
	}
	d.writeIndex = ^d.writeIndex
	return d
}

func (d *ManyToOne) Set(data []byte) {
	for {
		writeIndex := atomic.AddUint64(&d.writeIndex, 1)
		idx := writeIndex % uint64(len(d.buffer))
		old := atomic.LoadPointer(&d.buffer[idx])

		if old != nil &&
			(*bucket)(old) != nil &&
			(*bucket)(old).seq != writeIndex-uint64(len(d.buffer)) {
			continue
		}

		newBucket := &bucket{
			data: data,
			seq:  writeIndex,
		}

		if !atomic.CompareAndSwapPointer(&d.buffer[idx], old, unsafe.Pointer(newBucket)) {
			continue
		}

		return
	}
}

func (d *ManyToOne) TryNext() ([]byte, bool) {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	value, ok := d.tryNext(idx)
	if ok {
		atomic.AddUint64(&d.readIndex, 1)
	}
	return value, ok
}

func (d *ManyToOne) Next() []byte {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	result := d.pollBuffer(idx)
	atomic.AddUint64(&d.readIndex, 1)

	return result
}

func (d *ManyToOne) tryNext(idx uint64) ([]byte, bool) {
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

func (d *ManyToOne) pollBuffer(idx uint64) []byte {
	for {
		result, ok := d.tryNext(idx)

		if !ok {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		return result
	}
}
