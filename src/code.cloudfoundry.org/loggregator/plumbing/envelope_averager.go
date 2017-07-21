package plumbing

import (
	"sync/atomic"
	"time"
)

// EnvelopeAverager implements google.golang.org/grpc/stats.Handler and keeps
// track of the average envelope size emitted by a grpc client. It should
// be constructed with NewEnvelopeAverager.
//
// Note: Track is on a very "hot" path.
// Care should be taken while altering any algorithms, and performance
// must be considered.
type EnvelopeAverager struct {
	// storage must be accessed via atomics
	storage countAndTotal
}

// NewEnvelopeAverager creates a new EnvelopeAverager.
func NewEnvelopeAverager() *EnvelopeAverager {
	return &EnvelopeAverager{}
}

// Track takes the given envelope size (in bytes) to use in the current
// average calculation. It can be called by several go-routines.
func (a *EnvelopeAverager) Track(count, size int) {
	for {
		current := atomic.LoadUint64((*uint64)(&a.storage))
		newValue := countAndTotal(current).encodeDelta(uint16(count), uint64(size))

		if atomic.CompareAndSwapUint64((*uint64)(&a.storage), current, uint64(newValue)) {
			return
		}
	}
}

// Start invokes the given callback with the average envelope size of the past
// interval.
func (a *EnvelopeAverager) Start(interval time.Duration, f func(average float64)) {
	go func() {
		var prevCount uint16
		var prevTotal uint64
		for range time.Tick(interval) {
			current := atomic.LoadUint64((*uint64)(&a.storage))
			count, total := countAndTotal(current).decode()

			deltaCount := count - prevCount
			deltaTotal := total - prevTotal
			prevCount = count
			prevTotal = total

			if deltaCount == 0 {
				f(0)
				continue
			}

			f(float64(deltaTotal) / float64(deltaCount))
		}
	}()
}

// countAndTotal will encode both the count and total into a uint64's 8 bytes.
// The most significant 2 bytes are the count and the least significant 6 are
// the total. Endianness is irrelevant because the data is only being stored
// to local memory and not being transferred or persisted.
//
// [ count ] [ total ]
type countAndTotal uint64

func (u countAndTotal) encodeDelta(count uint16, total uint64) countAndTotal {
	currentCount, currentTotal := u.decode()
	count += currentCount
	total += currentTotal

	return countAndTotal((uint64(count) << 48) + total&0x0000FFFFFFFFFFFF)
}

func (u countAndTotal) decode() (count uint16, total uint64) {
	count = uint16((uint64(u) & 0xFFFF000000000000) >> 48)
	total = (uint64(u) & 0x0000FFFFFFFFFFFF)
	return count, total
}
