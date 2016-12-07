package diodes

import (
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cloudfoundry/sonde-go/events"
)

// ManyToOneEnvelope diode is optimal for many writers and a single
// reader.
type ManyToOneEnvelope struct {
	buffer     []unsafe.Pointer
	writeIndex uint64
	readIndex  uint64
	alerter    Alerter
}

type bucketEnvelope struct {
	data *events.Envelope
	seq  uint64
}

var NewManyToOneEnvelope = func(size int, alerter Alerter) *ManyToOneEnvelope {
	d := &ManyToOneEnvelope{
		buffer:  make([]unsafe.Pointer, size),
		alerter: alerter,
	}
	d.writeIndex = ^d.writeIndex
	return d
}

func (d *ManyToOneEnvelope) Set(data *events.Envelope) {
	for {
		writeIndex := atomic.AddUint64(&d.writeIndex, 1)
		idx := writeIndex % uint64(len(d.buffer))
		old := atomic.LoadPointer(&d.buffer[idx])

		if old != nil &&
			(*bucketEnvelope)(old) != nil &&
			(*bucketEnvelope)(old).seq != writeIndex-uint64(len(d.buffer)) {
			continue
		}

		newBucket := &bucketEnvelope{
			data: data,
			seq:  writeIndex,
		}

		if !atomic.CompareAndSwapPointer(&d.buffer[idx], old, unsafe.Pointer(newBucket)) {
			continue
		}

		return
	}
}

func (d *ManyToOneEnvelope) TryNext() (*events.Envelope, bool) {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	value, ok := d.tryNext(idx)
	if ok {
		atomic.AddUint64(&d.readIndex, 1)
	}
	return value, ok
}

func (d *ManyToOneEnvelope) Next() *events.Envelope {
	readIndex := atomic.LoadUint64(&d.readIndex)
	idx := readIndex % uint64(len(d.buffer))

	result := d.pollBuffer(idx)
	atomic.AddUint64(&d.readIndex, 1)

	return result
}

func (d *ManyToOneEnvelope) tryNext(idx uint64) (*events.Envelope, bool) {
	result := (*bucketEnvelope)(atomic.SwapPointer(&d.buffer[idx], nil))

	if result == nil {
		return nil, false
	}

	if result.seq > d.readIndex {
		d.alerter.Alert(int(result.seq - d.readIndex))
		atomic.StoreUint64(&d.readIndex, result.seq)
	}

	return result.data, true
}

func (d *ManyToOneEnvelope) pollBuffer(idx uint64) *events.Envelope {
	for {
		result, ok := d.tryNext(idx)

		if !ok {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		return result
	}
}
