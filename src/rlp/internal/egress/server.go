package egress

import (
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/batching"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
)

const (
	envelopeBufferSize = 10000
)

// HealthRegistrar provides an interface to record various counters.
type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

// Receiver creates a function which will receive envelopes on a stream.
type Receiver interface {
	Subscribe(ctx context.Context, req *loggregator_v2.EgressBatchRequest) (rx func() (*loggregator_v2.Envelope, error), err error)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
}

// Server represents a bridge between inbound data from the Receiver and
// outbound data on a gRPC stream.
type Server struct {
	receiver            Receiver
	egressMetric        *metricemitter.Counter
	droppedMetric       *metricemitter.Counter
	rejectedMetric      *metricemitter.Counter
	subscriptionsMetric *metricemitter.Gauge
	health              HealthRegistrar
	ctx                 context.Context
	batchSize           int
	batchInterval       time.Duration
	maxStreams          int64
	subscriptions       int64
}

// NewServer is the preferred way to create a new Server.
func NewServer(
	r Receiver,
	m MetricClient,
	h HealthRegistrar,
	c context.Context,
	batchSize int,
	batchInterval time.Duration,
	opts ...ServerOption,
) *Server {
	egressMetric := m.NewCounter("egress",
		metricemitter.WithVersion(2, 0),
	)

	droppedMetric := m.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{
			"direction": "egress",
		}),
	)

	rejectedMetric := m.NewCounter("rejected_streams",
		metricemitter.WithVersion(2, 0),
	)

	subscriptionsMetric := m.NewGauge("subscriptions", "total",
		metricemitter.WithVersion(2, 0),
	)

	s := &Server{
		receiver:            r,
		egressMetric:        egressMetric,
		droppedMetric:       droppedMetric,
		rejectedMetric:      rejectedMetric,
		subscriptionsMetric: subscriptionsMetric,
		health:              h,
		ctx:                 c,
		batchSize:           batchSize,
		batchInterval:       batchInterval,
		maxStreams:          500,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// ServerOption represents a function that cancel configure an RLP egress server.
type ServerOption func(*Server)

// WithMaxStreams specifies the maximum streams allowed by the RLP egress
// server.
func WithMaxStreams(conn int64) ServerOption {
	return func(s *Server) {
		s.maxStreams = conn
	}
}

// Receiver implements the loggregator-api V2 gRPC interface for receiving
// envelopes from upstream connections.
func (s *Server) Receiver(r *loggregator_v2.EgressRequest, srv loggregator_v2.Egress_ReceiverServer) error {
	s.health.Inc("subscriptionCount")
	defer s.health.Dec("subscriptionCount")

	s.subscriptionsMetric.Increment(1)
	defer s.subscriptionsMetric.Decrement(1)

	subCount := atomic.AddInt64(&s.subscriptions, 1)
	defer atomic.AddInt64(&s.subscriptions, -1)

	if subCount > s.maxStreams {
		s.rejectedMetric.Increment(1)
		return status.Errorf(codes.ResourceExhausted, "unable to create stream, max egress streams reached: %d", s.maxStreams)
	}

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	buffer := make(chan *loggregator_v2.Envelope, envelopeBufferSize)

	r.Selectors = s.convergeSelectors(r.GetLegacySelector(), r.GetSelectors())
	r.LegacySelector = nil

	if len(r.Selectors) == 0 {
		return status.Errorf(codes.InvalidArgument, "Selectors cannot be empty")
	}
	for _, s := range r.Selectors {
		if s.Message == nil {
			return status.Errorf(codes.InvalidArgument, "Selectors must have a Message")
		}
	}

	go func() {
		select {
		case <-s.ctx.Done():
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()

	br := &loggregator_v2.EgressBatchRequest{
		ShardId:          r.GetShardId(),
		Selectors:        r.GetSelectors(),
		UsePreferredTags: r.GetUsePreferredTags(),
	}

	rx, err := s.receiver.Subscribe(ctx, br)
	if err != nil {
		log.Printf("Unable to setup subscription: %s", err)
		return fmt.Errorf("unable to setup subscription")
	}

	go s.consumeReceiver(r.UsePreferredTags, buffer, rx, cancel)

	for data := range buffer {
		if err := srv.Send(data); err != nil {
			log.Printf("Send error: %s", err)
			return io.ErrUnexpectedEOF
		}

		// metric-documentation-v2: (loggregator.rlp.egress) Number of v2
		// envelopes sent to RLP consumers.
		s.egressMetric.Increment(1)
	}

	return nil
}

// BatchedReceiver implements the loggregator-api V2 gRPC interface for
// receiving batches of envelopes. Envelopes will be written to the egress
// batched receiver server whenever the configured interval or configured
// batch size is exceeded.
func (s *Server) BatchedReceiver(r *loggregator_v2.EgressBatchRequest, srv loggregator_v2.Egress_BatchedReceiverServer) error {
	s.health.Inc("subscriptionCount")
	defer s.health.Dec("subscriptionCount")

	s.subscriptionsMetric.Increment(1)
	defer s.subscriptionsMetric.Decrement(1)

	subCount := atomic.AddInt64(&s.subscriptions, 1)
	defer func() { atomic.AddInt64(&s.subscriptions, -1) }()

	if subCount > s.maxStreams {
		s.rejectedMetric.Increment(1)
		return status.Errorf(codes.ResourceExhausted, "unable to create stream, max egress streams reached: %d", s.maxStreams)
	}

	r.Selectors = s.convergeSelectors(r.GetLegacySelector(), r.GetSelectors())
	r.LegacySelector = nil

	if len(r.Selectors) == 0 {
		return status.Errorf(codes.InvalidArgument, "Selectors cannot be empty")
	}
	for _, s := range r.Selectors {
		if s.Message == nil {
			return status.Errorf(codes.InvalidArgument, "Selectors must have a Message")
		}
	}

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	buffer := make(chan *loggregator_v2.Envelope, envelopeBufferSize)

	go func() {
		select {
		case <-s.ctx.Done():
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()

	rx, err := s.receiver.Subscribe(ctx, r)
	if err != nil {
		log.Printf("Unable to setup subscription: %s", err)
		return fmt.Errorf("unable to setup subscription")
	}

	receiveErrorStream := make(chan error, 1)
	go s.consumeBatchReceiver(r.UsePreferredTags, buffer, receiveErrorStream, rx, cancel)

	senderErrorStream := make(chan error, 1)
	batcher := batching.NewV2EnvelopeBatcher(
		s.batchSize,
		s.batchInterval,
		&batchWriter{
			srv:          srv,
			errStream:    senderErrorStream,
			egressMetric: s.egressMetric,
		},
	)

	resetDuration := 100 * time.Millisecond
	timer := time.NewTimer(resetDuration)
	for {
		select {
		case data, ok := <-buffer:
			if !ok {
				continue
			}
			batcher.Write(data)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(resetDuration)
		case <-senderErrorStream:
			return io.ErrUnexpectedEOF
		case <-receiveErrorStream:
			for d := range buffer {
				batcher.Write(d)
			}
			batcher.ForcedFlush()

			return nil
		case <-timer.C:
			batcher.Flush()
			timer.Reset(resetDuration)
		}
	}

	return nil
}

// convergeSelectors takes in any LegacySelector on the request as well as
// Selectors and converts LegacySelector into a Selector based on Selector
// hierarchy.
func (s *Server) convergeSelectors(
	legacy *loggregator_v2.Selector,
	selectors []*loggregator_v2.Selector,
) []*loggregator_v2.Selector {
	if legacy != nil && len(selectors) > 0 {
		// Both would be set by the consumer for upgrade path purposes.
		// The contract should be to assume that the Selectors encompasses
		// the LegacySelector. Therefore, just ignore the LegacySelector.
		return selectors
	}

	if legacy != nil {
		return []*loggregator_v2.Selector{legacy}
	}

	return selectors
}

type batchWriter struct {
	srv          loggregator_v2.Egress_BatchedReceiverServer
	errStream    chan<- error
	egressMetric *metricemitter.Counter
}

func (b *batchWriter) Write(batch []*loggregator_v2.Envelope) {
	err := b.srv.Send(&loggregator_v2.EnvelopeBatch{Batch: batch})
	if err != nil {
		select {
		case b.errStream <- err:
		default:
		}
		return
	}

	// metric-documentation-v2: (loggregator.rlp.egress) Number of v2
	// envelopes sent to RLP consumers.
	b.egressMetric.Increment(uint64(len(batch)))
}

func (s *Server) consumeBatchReceiver(
	usePreferred bool,
	buffer chan<- *loggregator_v2.Envelope,
	errorStream chan<- error,
	rx func() (*loggregator_v2.Envelope, error),
	cancel func(),
) {

	defer cancel()
	defer close(buffer)

	for {
		e, err := rx()
		if err == io.EOF {
			errorStream <- err
			break
		}

		if err != nil {
			log.Printf("Subscribe error: %s", err)
			errorStream <- err
			break
		}

		s.convergeTags(usePreferred, e)

		select {
		case buffer <- e:
		default:
			// metric-documentation-v2: (loggregator.rlp.dropped) Number of v2
			// envelopes dropped while egressing to a consumer.
			s.droppedMetric.Increment(1)
		}
	}
}

func (s *Server) consumeReceiver(
	usePreferred bool,
	buffer chan<- *loggregator_v2.Envelope,
	rx func() (*loggregator_v2.Envelope, error),
	cancel func(),
) {

	defer cancel()
	defer close(buffer)

	for {
		e, err := rx()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Printf("Subscribe error: %s", err)
			break
		}

		s.convergeTags(usePreferred, e)

		select {
		case buffer <- e:
		default:
			// metric-documentation-v2: (loggregator.rlp.dropped) Number of v2
			// envelopes dropped while egressing to a consumer.
			s.droppedMetric.Increment(1)
		}
	}
}

func (s *Server) convergeTags(usePreferred bool, e *loggregator_v2.Envelope) {
	if usePreferred {
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}

		for name, value := range e.GetDeprecatedTags() {
			switch x := value.Data.(type) {
			case *loggregator_v2.Value_Decimal:
				e.GetTags()[name] = fmt.Sprint(x.Decimal)
			case *loggregator_v2.Value_Integer:
				e.GetTags()[name] = fmt.Sprint(x.Integer)
			case *loggregator_v2.Value_Text:
				e.GetTags()[name] = x.Text
			}
		}
		e.DeprecatedTags = nil
		return
	}

	if e.DeprecatedTags == nil {
		e.DeprecatedTags = make(map[string]*loggregator_v2.Value)
	}

	for name, value := range e.GetTags() {
		e.GetDeprecatedTags()[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{
				Text: value,
			},
		}
	}
	e.Tags = nil
}
