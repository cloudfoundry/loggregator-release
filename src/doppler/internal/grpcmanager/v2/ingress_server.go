package v2

import (
	"metricemitter"
	"plumbing/conversion"
	plumbing "plumbing/v2"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
)

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
}

func NewIngressServer(
	envelopeBuffer DataSetter,
	batcher Batcher,
	metricClient metricemitter.MetricClient,
) *IngressServer {
	ingressMetric := metricClient.NewCounter("ingress",
		metricemitter.WithVersion(2, 0),
	)

	return &IngressServer{
		envelopeBuffer: envelopeBuffer,
		batcher:        batcher,
		ingressMetric:  ingressMetric,
	}
}

func (i IngressServer) BatchSender(s plumbing.DopplerIngress_BatchSenderServer) error {
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

				// metric-documentation-v2: (loggregator.doppler.ingress) Number of received
				// envelopes from Metron on Doppler's v2 gRPC server
				i.ingressMetric.Increment(1)
			}
		}
	}
}

func (i IngressServer) Sender(s plumbing.DopplerIngress_SenderServer) error {
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

			// metric-documentation-v2: (loggregator.doppler.ingress) Number of received
			// envelopes from Metron on Doppler's v2 gRPC server
			i.ingressMetric.Increment(1)
		}
	}
}
