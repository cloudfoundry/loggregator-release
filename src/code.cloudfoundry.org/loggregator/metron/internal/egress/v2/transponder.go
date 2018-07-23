package v2

import (
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/batching"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Nexter interface {
	TryNext() (*v2.Envelope, bool)
}

type Writer interface {
	Write(msgs []*v2.Envelope) error
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

type Transponder struct {
	nexter        Nexter
	writer        Writer
	tags          map[string]string
	batcher       *batching.V2EnvelopeBatcher
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
	b := batching.NewV2EnvelopeBatcher(
		t.batchSize,
		t.batchInterval,
		batching.V2EnvelopeWriterFunc(t.write),
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

func (t *Transponder) write(batch []*v2.Envelope) {
	for _, e := range batch {
		t.addTags(e)
	}

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

func (t *Transponder) addTags(e *v2.Envelope) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}

	// Move deprecated tags to tags.
	for k, v := range e.GetDeprecatedTags() {
		switch v.Data.(type) {
		case *v2.Value_Text:
			e.Tags[k] = v.GetText()
		case *v2.Value_Integer:
			e.Tags[k] = strconv.FormatInt(v.GetInteger(), 10)
		case *v2.Value_Decimal:
			e.Tags[k] = strconv.FormatFloat(v.GetDecimal(), 'f', -1, 64)
		default:
			e.Tags[k] = v.String()
		}
	}

	for k, v := range t.tags {
		if _, ok := e.Tags[k]; !ok {
			e.Tags[k] = v
		}
	}

	e.DeprecatedTags = nil
}
