package batching

import "time"

// Batcher will accept messages and invoke the Writer when the batch
// requirements have been fulfilled (either batch size or interval have been
// exceeded). Batcher should be created with NewBatcher().
type Batcher struct {
	w        Writer
	size     int
	interval time.Duration
	batch    []interface{}
	lastSent time.Time
}

// Writer is used to submit the completed batch. The batch may be partial if
// the interval lapsed instead of filling the batch.
type Writer interface {
	// Write submits the batch.
	Write(batch []interface{})
}

// WriterFunc is an adapter to allow ordinary functions to be a Writer.
type WriterFunc func(batch []interface{})

// Write implements Writer.
func (f WriterFunc) Write(batch []interface{}) {
	f(batch)
}

// NewBatcher creates a new Batcher. It is recommenended to use a wrapper type
// such as NewByteBatcher or NewV2EnvelopeBatcher vs using this directly.
func NewBatcher(size int, interval time.Duration, writer Writer) *Batcher {
	return &Batcher{
		size:     size,
		interval: interval,
		w:        writer,
		lastSent: time.Now(),
	}
}

// Write stores data to the batch. It will not submit the batch to the writer
// until either the batch has been filled, or the interval has lapsed. NOTE:
// Write is *not* thread safe and should be called by the same goroutine that
// calls Flush.
func (b *Batcher) Write(data interface{}) {
	b.batch = append(b.batch, data)
	if b.partialBatch() && b.partialInterval() {
		return
	}

	b.writeBatch()
}

// ForcedFlush bypasses the batch interval and batch size checks and writes
// immediately.
func (b *Batcher) ForcedFlush() {
	b.writeBatch()
}

// Flush will write a partial batch if there is data and the interval has
// lapsed. Otherwise it is a NOP. This method should be called freqently to
// make sure batches do not stick around for long periods of time. As a result
// it would be a bad idea to call Flush after an operation that might block
// for an un-specified amount of time. NOTE: Flush is *not* thread safe and
// should be called by the same goroutine that calls Write.
func (b *Batcher) Flush() {
	if b.partialInterval() {
		return
	}

	b.writeBatch()
}

// writeBatch writes the batch (if any) to the writer and resets the batch and
// interval.
func (b *Batcher) writeBatch() {
	if len(b.batch) == 0 {
		return
	}

	b.w.Write(b.batch)
	b.batch = nil
	b.lastSent = time.Now()
}

func (b *Batcher) partialBatch() bool {
	return len(b.batch) < b.size
}

func (b *Batcher) partialInterval() bool {
	return time.Since(b.lastSent) < b.interval
}
