package v2

import (
	"log"
	"time"

	gendiode "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/diodes"
	"code.cloudfoundry.org/loggregator-release/metricemitter"
	"code.cloudfoundry.org/loggregator-release/plumbing/batching"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Subscriber registers stream DataSetters to accept reads.
type Subscriber interface {
	Subscribe(req *loggregator_v2.EgressBatchRequest, setter DataSetter) (unsubscribe func())
}

// DataSetter accepts writes of v2.Envelopes
// TODO: This could be a named function. This will be a performance bump.
type DataSetter interface {
	Set(*loggregator_v2.Envelope)
}

// EgressServer implements the loggregator_v2.EgressServer interface.
type EgressServer struct {
	loggregator_v2.EgressServer

	subscriber          Subscriber
	egressMetric        *metricemitter.Counter
	droppedMetric       *metricemitter.Counter
	subscriptionsMetric *metricemitter.Gauge
	batchInterval       time.Duration
	batchSize           uint
}

// NewEgressServer is the constructor for EgressServer.
func NewEgressServer(
	s Subscriber,
	m MetricClient,
	droppedMetric *metricemitter.Counter,
	subscriptionsMetric *metricemitter.Gauge,
	batchInterval time.Duration,
	batchSize uint,
) *EgressServer {
	// metric-documentation-v2: (loggregator.doppler.egress) Number of
	// envelopes read from a diode to be sent to subscriptions.
	egressMetric := m.NewCounter("egress",
		metricemitter.WithVersion(2, 0),
	)

	return &EgressServer{
		subscriber:          s,
		egressMetric:        egressMetric,
		droppedMetric:       droppedMetric,
		subscriptionsMetric: subscriptionsMetric,
		batchInterval:       batchInterval,
		batchSize:           batchSize,
	}
}

// Alert logs dropped message counts to stderr.
func (s *EgressServer) Alert(missed int) {
	s.droppedMetric.Increment(uint64(missed))
}

// Receiver implements loggregator_v2.EgressServer.
func (s *EgressServer) Receiver(
	req *loggregator_v2.EgressRequest,
	sender loggregator_v2.Egress_ReceiverServer,
) error {
	return status.Errorf(codes.Unimplemented, "use BatchedReceiver instead")
}

// BatchedReceiver implements loggregator_v2.EgressServer.
func (s *EgressServer) BatchedReceiver(
	req *loggregator_v2.EgressBatchRequest,
	sender loggregator_v2.Egress_BatchedReceiverServer,
) error {
	s.subscriptionsMetric.Increment(1.0)
	defer s.subscriptionsMetric.Decrement(1.0)

	d := diodes.NewOneToOneWaiterEnvelopeV2(
		1000,
		gendiode.AlertFunc(func(missed int) {
			log.Printf("Dropped %d envelopes (v2 buffer) ShardID: %s", missed, req.ShardId)
			s.Alert(missed)
		}),
		gendiode.WithWaiterContext(sender.Context()),
	)
	cancel := s.subscriber.Subscribe(req, d)
	defer cancel()

	errStream := make(chan error, 1)
	batcher := batching.NewV2EnvelopeBatcher(
		int(s.batchSize),
		s.batchInterval,
		&batchWriter{
			sender:       sender,
			errStream:    errStream,
			egressMetric: s.egressMetric,
		},
	)

	c := make(chan *loggregator_v2.Envelope)
	go func() {
		for {
			env := d.Next()
			if env == nil {
				return
			}

			select {
			case c <- env:
			case <-sender.Context().Done():
				return
			}
		}
	}()

	resetDuration := 250 * time.Millisecond
	timer := time.NewTimer(resetDuration)
	for {
		select {
		case <-sender.Context().Done():
			return sender.Context().Err()
		case err := <-errStream:
			return err
		case <-timer.C:
			batcher.ForcedFlush()
			// Don't call stop like the documentation recommends because this
			// case implies the timer has infact been triggered.
			timer.Reset(resetDuration)
		case env := <-c:
			batcher.Write(env)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(resetDuration)
		}
	}
}

type batchWriter struct {
	sender       loggregator_v2.Egress_BatchedReceiverServer
	errStream    chan<- error
	egressMetric *metricemitter.Counter
}

// Write adds an entry to the batch. If the batch conditions are met, the
// batch is flushed.
func (b *batchWriter) Write(batch []*loggregator_v2.Envelope) {
	err := b.sender.Send(&loggregator_v2.EnvelopeBatch{
		Batch: batch,
	})
	if err != nil {
		b.errStream <- err
		return
	}
	b.egressMetric.Increment(uint64(len(batch)))
}
