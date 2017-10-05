package v2

import (
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
)

// HealthRegistrar describes the health interface for keeping track of various
// values.
type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

type DopplerIngress_SenderServer interface {
	plumbing.DopplerIngress_SenderServer
}

type Batcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type IngressServer struct {
	v1Buf         *diodes.ManyToOneEnvelope
	v2Buf         *diodes.ManyToOneEnvelopeV2
	batcher       Batcher
	ingressMetric *metricemitter.Counter
	health        HealthRegistrar
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

func NewIngressServer(
	v1Buf *diodes.ManyToOneEnvelope,
	v2Buf *diodes.ManyToOneEnvelopeV2,
	batcher Batcher,
	metricClient MetricClient,
	health HealthRegistrar,
) *IngressServer {
	ingressMetric := metricClient.NewCounter("ingress",
		metricemitter.WithVersion(2, 0),
	)

	return &IngressServer{
		v1Buf:         v1Buf,
		v2Buf:         v2Buf,
		batcher:       batcher,
		ingressMetric: ingressMetric,
		health:        health,
	}
}

func (i IngressServer) BatchSender(s plumbing.DopplerIngress_BatchSenderServer) error {
	i.health.Inc("ingressStreamCount")
	defer i.health.Dec("ingressStreamCount")

	for {
		v2eBatch, err := s.Recv()
		if err != nil {
			return err
		}

		for _, v2e := range v2eBatch.Batch {
			i.v2Buf.Set(v2e)
			envelopes := conversion.ToV1(v2e)

			for _, v1e := range envelopes {
				if v1e == nil || v1e.EventType == nil {
					continue
				}

				i.v1Buf.Set(v1e)

				// metric-documentation-v1: (listeners.totalReceivedMessageCount)
				// Total number of messages received by doppler.
				i.batcher.BatchCounter("listeners.totalReceivedMessageCount").
					Increment()

				// metric-documentation-v2: (loggregator.doppler.ingress) Number of received
				// envelopes from Metron on Doppler's v2 gRPC server
				i.ingressMetric.Increment(1)
			}
		}
	}
}

// TODO Remove the Sender method onces we are certain all Metrons are using
// the BatchSender method
func (i IngressServer) Sender(s plumbing.DopplerIngress_SenderServer) error {
	i.health.Inc("ingressStreamCount")
	defer i.health.Dec("ingressStreamCount")

	for {
		v2e, err := s.Recv()
		if err != nil {
			return err
		}

		i.v2Buf.Set(v2e)
		envelopes := conversion.ToV1(v2e)

		for _, v1e := range envelopes {
			if v1e == nil || v1e.EventType == nil {
				continue
			}

			i.v1Buf.Set(v1e)

			// metric-documentation-v1: (listeners.totalReceivedMessageCount)
			// Total number of messages received by doppler.
			i.batcher.BatchCounter("listeners.totalReceivedMessageCount").
				Increment()

			// metric-documentation-v2: (loggregator.doppler.ingress) Number of received
			// envelopes from Metron on Doppler's v2 gRPC server
			i.ingressMetric.Increment(1)
		}
	}
}
