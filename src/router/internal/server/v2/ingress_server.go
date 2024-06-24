package v2

import (
	"code.cloudfoundry.org/go-loggregator/v10/conversion"
	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/diodes"
	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

type IngressServer struct {
	loggregator_v2.IngressServer

	v1Buf         *diodes.ManyToOneEnvelope
	v2Buf         *diodes.ManyToOneEnvelopeV2
	ingressMetric *metricemitter.Counter
}

func NewIngressServer(
	v1Buf *diodes.ManyToOneEnvelope,
	v2Buf *diodes.ManyToOneEnvelopeV2,
	ingressMetric *metricemitter.Counter,
) *IngressServer {
	return &IngressServer{
		v1Buf:         v1Buf,
		v2Buf:         v2Buf,
		ingressMetric: ingressMetric,
	}
}

func (i IngressServer) Send(
	_ context.Context,
	_ *loggregator_v2.EnvelopeBatch,
) (*loggregator_v2.SendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "this endpoint is not yet implemented")
}

func (i IngressServer) BatchSender(s loggregator_v2.Ingress_BatchSenderServer) error {
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
				i.ingressMetric.Increment(1)
			}
		}
	}
}

// TODO Remove the Sender method onces we are certain all Metrons are using
// the BatchSender method
func (i IngressServer) Sender(s loggregator_v2.Ingress_SenderServer) error {
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
			i.ingressMetric.Increment(1)
		}
	}
}
