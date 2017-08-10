package batching

import "time"

// ByteBatcher batches slices of bytes.
type ByteBatcher struct {
	*Batcher
}

// ByteWriter is used to submit the completed batch of slices of bytes. The
// batch may be partial if the interval lapsed instead of filling the batch.
type ByteWriter interface {
	// Write submits the batch.
	Write(batch [][]byte)
}

// ByteWriterFunc is an adapter to allow ordinary functions to be a ByteWriter.
type ByteWriterFunc func(batch [][]byte)

// Write implements ByteWriter.
func (f ByteWriterFunc) Write(batch [][]byte) {
	f(batch)
}

// NewByteBatcher creates a new ByteBatcher.
func NewByteBatcher(size int, interval time.Duration, writer ByteWriter) *ByteBatcher {
	genWriter := WriterFunc(func(batch []interface{}) {
		byteBatch := make([][]byte, 0, len(batch))
		for _, element := range batch {
			byteBatch = append(byteBatch, element.([]byte))
		}
		writer.Write(byteBatch)
	})
	return &ByteBatcher{
		Batcher: NewBatcher(size, interval, genWriter),
	}
}

// Write stores data to the batch. It will not submit the batch to the writer
// until either the batch has been filled, or the interval has lapsed. NOTE:
// Write is *not* thread safe and should be called by the same goroutine that
// calls Flush.
func (b *ByteBatcher) Write(data []byte) {
	b.Batcher.Write(data)
}
