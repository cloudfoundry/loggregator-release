package diodes

import (
	v2 "plumbing/v2"
	"sync/atomic"
	"time"
	"unsafe"
)

// ManyToOneEnvelopeV2 diode is optimal for many writers and a single
// reader.
type ManyToOneEnvelopeV2 struct {
	buffer     []unsafe.Pointer
	writeIndex uint64
	readIndex  uint64
	alerter    Alerter
}

type bucketEnvelopeV2 struct {
	data *v2.Envelope
	seq  uint64
}

var NewManyToOneEnvelopeV2 = func(size int, alerter Alerter) *ManyToOneEnvelopeV2 {
	d := &ManyToOneEnvelopeV2{
		buffer:  make([]unsafe.Pointer, size),
		alerter: alerter,
	}
	d.writeIndex = ^d.writeIndex
	return d
}

func (d *ManyToOneEnvelopeV2) Set(data *v2.Envelope) {
	for {
		writeIndex := atomic.AddUint64(&d.writeIndex, 1)
		idx := writeIndex % uint64(len(d.buffer))
		old := atomic.LoadPointer(&d.buffer[idx])

		if old != nil &&
			(*bucketEnvelopeV2)(old) != nil &&
			(*bucketEnvelopeV2)(old).seq != writeIndex-uint64(len(d.buffer)) {
			continue
		}

		newBucket := &bucketEnvelopeV2{
			data: data,
			seq:  writeIndex,
		}

		if !atomic.CompareAndSwapPointer(&d.buffer[idx], old, unsafe.Pointer(newBucket)) {
			continue
		}

		return
	}
}

func (d *ManyToOneEnvelopeV2) TryNext() (*v2.Envelope, bool) {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	value, ok := d.tryNext(idx)
	if ok {
		atomic.AddUint64(&d.readIndex, 1)
	}
	return value, ok
}

func (d *ManyToOneEnvelopeV2) Next() *v2.Envelope {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	result := d.pollBuffer(idx)
	atomic.AddUint64(&d.readIndex, 1)

	return result
}

func (d *ManyToOneEnvelopeV2) tryNext(idx uint64) (*v2.Envelope, bool) {
	result := (*bucketEnvelopeV2)(atomic.SwapPointer(&d.buffer[idx], nil))

	if result == nil {
		return nil, false
	}

	if result.seq > d.readIndex {
		d.alerter.Alert(int(result.seq - d.readIndex))
		atomic.StoreUint64(&d.readIndex, result.seq)
	}

	return result.data, true
}

func (d *ManyToOneEnvelopeV2) pollBuffer(idx uint64) *v2.Envelope {
	for {
		result, ok := d.tryNext(idx)

		if !ok {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		return result
	}
}
