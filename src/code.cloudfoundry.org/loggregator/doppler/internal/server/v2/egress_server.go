package v2

import (
	"log"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/plumbing/batching"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
	subscriber    Subscriber
	health        HealthRegistrar
	batchInverval time.Duration
	batchSize     uint
}

// NewEgressServer is the constructor for EgressServer.
func NewEgressServer(
	s Subscriber,
	h HealthRegistrar,
	batchInverval time.Duration,
	batchSize uint,
) *EgressServer {
	return &EgressServer{
		subscriber:    s,
		health:        h,
		batchInverval: batchInverval,
		batchSize:     batchSize,
	}
}

// Alert logs dropped message counts to stderr.
func (*EgressServer) Alert(missed int) {
	// TODO: Should we have a metric?
	log.Printf("Dropped %d envelopes (v2 Egress Server)", missed)
}

// Receiver implements loggregator_v2.EgressServer.
func (s *EgressServer) Receiver(
	req *loggregator_v2.EgressRequest,
	sender loggregator_v2.Egress_ReceiverServer,
) error {
	return grpc.Errorf(codes.Unimplemented, "use BatchedReceiver instead")
}

// BatchedReceiver implements loggregator_v2.EgressServer.
func (s *EgressServer) BatchedReceiver(
	req *loggregator_v2.EgressBatchRequest,
	sender loggregator_v2.Egress_BatchedReceiverServer,
) error {
	s.health.Inc("subscriptionCount")
	defer s.health.Dec("subscriptionCount")

	d := diodes.NewOneToOneEnvelopeV2(1000, s)
	cancel := s.subscriber.Subscribe(req, d)
	defer cancel()

	errStream := make(chan error, 1)
	batcher := batching.NewV2EnvelopeBatcher(
		int(s.batchSize),
		s.batchInverval,
		&batchWriter{sender: sender, errStream: errStream},
	)

	for {
		select {
		case <-sender.Context().Done():
			return sender.Context().Err()
		case err := <-errStream:
			return err
		default:
			data, ok := d.TryNext()
			if !ok {
				batcher.ForcedFlush()
				time.Sleep(10 * time.Millisecond)
				continue
			}

			batcher.Write(data)
		}
	}
}

type batchWriter struct {
	sender    loggregator_v2.Egress_BatchedReceiverServer
	errStream chan<- error
}

// Write adds an entry to the batch. If the batch conditions are met, the
// batch is flushed.
func (b *batchWriter) Write(batch []*loggregator_v2.Envelope) {
	err := b.sender.Send(&loggregator_v2.EnvelopeBatch{
		Batch: batch,
	})
	if err != nil {
		b.errStream <- err
	}
}
