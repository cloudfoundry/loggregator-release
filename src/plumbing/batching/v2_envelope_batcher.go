package batching

import (
	"time"

	"code.cloudfoundry.org/go-batching"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

// V2EnvelopeBatcher batches slices of bytes.
type V2EnvelopeBatcher struct {
	*batching.Batcher
}

// V2EnvelopeWriter is used to submit the completed batch of v2 envelopes. The
// batch may be partial if the interval lapsed instead of filling the batch.
type V2EnvelopeWriter interface {
	// Write submits the batch.
	Write(batch []*loggregator_v2.Envelope)
}

// V2EnvelopeWriterFunc is an adapter to allow ordinary functions to be a
// V2EnvelopeWriter.
type V2EnvelopeWriterFunc func(batch []*loggregator_v2.Envelope)

// Write implements V2EnvelopeWriter.
func (f V2EnvelopeWriterFunc) Write(batch []*loggregator_v2.Envelope) {
	f(batch)
}

// NewV2EnvelopeBatcher creates a new ByteBatcher.
func NewV2EnvelopeBatcher(size int, interval time.Duration, writer V2EnvelopeWriter) *V2EnvelopeBatcher {
	genWriter := batching.WriterFunc(func(batch []interface{}) {
		envBatch := make([]*loggregator_v2.Envelope, 0, len(batch))
		for _, element := range batch {
			envBatch = append(envBatch, element.(*loggregator_v2.Envelope))
		}
		writer.Write(envBatch)
	})
	return &V2EnvelopeBatcher{
		Batcher: batching.NewBatcher(size, interval, genWriter),
	}
}

// Write stores data to the batch. It will not submit the batch to the writer
// until either the batch has been filled, or the interval has lapsed. NOTE:
// Write is *not* thread safe and should be called by the same goroutine that
// calls Flush.
func (b *V2EnvelopeBatcher) Write(data *loggregator_v2.Envelope) {
	b.Batcher.Write(data)
}
