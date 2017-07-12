package v2

import (
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
)

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

type DataSetter interface {
	Set(data *events.Envelope)
}

type IngressServer struct {
	envelopeBuffer DataSetter
	batcher        Batcher
	ingressMetric  *metricemitter.Counter
	health         HealthRegistrar
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

func NewIngressServer(
	envelopeBuffer DataSetter,
	batcher Batcher,
	metricClient MetricClient,
	health HealthRegistrar,
) *IngressServer {
	ingressMetric := metricClient.NewCounter("ingress",
		metricemitter.WithVersion(2, 0),
	)

	return &IngressServer{
		envelopeBuffer: envelopeBuffer,
		batcher:        batcher,
		ingressMetric:  ingressMetric,
		health:         health,
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
			envelopes := conversion.ToV1(v2e)
			for _, v1e := range envelopes {
				if v1e == nil || v1e.EventType == nil {
					continue
				}

				i.envelopeBuffer.Set(v1e)

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

func (i IngressServer) Sender(s plumbing.DopplerIngress_SenderServer) error {
	i.health.Inc("ingressStreamCount")
	defer i.health.Dec("ingressStreamCount")

	for {
		v2e, err := s.Recv()
		if err != nil {
			return err
		}

		envelopes := conversion.ToV1(v2e)
		for _, v1e := range envelopes {
			if v1e == nil || v1e.EventType == nil {
				continue
			}

			i.envelopeBuffer.Set(v1e)

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
