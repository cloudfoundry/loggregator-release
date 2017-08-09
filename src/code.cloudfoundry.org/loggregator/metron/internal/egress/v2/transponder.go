package v2

import (
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/batching"
	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Nexter interface {
	TryNext() (*plumbing.Envelope, bool)
}

type Writer interface {
	Write(msgs []*plumbing.Envelope) error
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

type Transponder struct {
	nexter        Nexter
	writer        Writer
	tags          map[string]string
	batcher       *batching.Batcher
	batchSize     int
	batchInterval time.Duration
	droppedMetric *metricemitter.Counter
	egressMetric  *metricemitter.Counter
}

func NewTransponder(
	n Nexter,
	w Writer,
	tags map[string]string,
	batchSize int,
	batchInterval time.Duration,
	metricClient MetricClient,
) *Transponder {
	droppedMetric := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "egress"}),
	)

	egressMetric := metricClient.NewCounter("egress",
		metricemitter.WithVersion(2, 0),
	)

	return &Transponder{
		nexter:        n,
		writer:        w,
		tags:          tags,
		droppedMetric: droppedMetric,
		egressMetric:  egressMetric,
		batchSize:     batchSize,
		batchInterval: batchInterval,
	}
}

func (t *Transponder) Start() {
	b := batching.NewBatcher(
		t.batchSize,
		t.batchInterval,
		batching.WriterFunc(t.convertAndWrite),
	)

	for {
		envelope, ok := t.nexter.TryNext()
		if !ok {
			b.Flush()
			time.Sleep(10 * time.Millisecond)
			continue
		}

		b.Write(envelope)
	}
}

func (t *Transponder) convertAndWrite(batch []interface{}) {
	envelopes := make([]*plumbing.Envelope, 0, len(batch))
	for _, b := range batch {
		e := b.(*plumbing.Envelope)
		t.addTags(e)
		envelopes = append(envelopes, e)
	}
	t.write(envelopes)
}

func (t *Transponder) write(batch []*plumbing.Envelope) {
	if err := t.writer.Write(batch); err != nil {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
		// dropped when failing to write to Dopplers v2 API
		t.droppedMetric.Increment(uint64(len(batch)))
		return
	}

	// metric-documentation-v2: (loggregator.metron.egress)
	// Number of messages written to Doppler's v2 API
	t.egressMetric.Increment(uint64(len(batch)))
}

func (t *Transponder) addTags(e *plumbing.Envelope) {
	if e.DeprecatedTags == nil {
		e.DeprecatedTags = make(map[string]*plumbing.Value)
	}

	// Move non-deprecated tags to deprecated tags. This is required
	// for backwards compatibility purposes and should be removed once
	// deprecated tags are fully removed.
	for k, v := range e.GetTags() {
		e.DeprecatedTags[k] = &plumbing.Value{
			Data: &plumbing.Value_Text{
				Text: v,
			},
		}
	}

	for k, v := range t.tags {
		if _, ok := e.DeprecatedTags[k]; !ok {
			e.DeprecatedTags[k] = &plumbing.Value{
				Data: &plumbing.Value_Text{
					Text: v,
				},
			}
		}
	}
}
