package ingress

import (
	"log"

	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"golang.org/x/net/context"
)

type Subscriber interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (recv func() ([]byte, error), err error)
}

type EnvelopeConverter interface {
	Convert(data []byte, usePreferredTags bool) (*v2.Envelope, error)
}

type RequestConverter interface {
	Convert(req *v2.EgressRequest) *plumbing.SubscriptionRequest
}

type Receiver struct {
	envConverter EnvelopeConverter
	reqConverter RequestConverter
	subscriber   Subscriber
}

func NewReceiver(c EnvelopeConverter, r RequestConverter, s Subscriber) *Receiver {
	return &Receiver{
		envConverter: c,
		reqConverter: r,
		subscriber:   s,
	}
}

func (r *Receiver) Receive(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error) {
	v1Rx, err := r.subscriber.Subscribe(ctx, r.reqConverter.Convert(req))
	if err != nil {
		return nil, err
	}

	return func() (*v2.Envelope, error) {
		data, err := v1Rx()
		if err != nil {
			log.Printf("Subscription receiver error: %s", err)
			return nil, err
		}

		v2e, err := r.envConverter.Convert(data, req.UsePreferredTags)
		if err != nil {
			log.Printf("V1->V2 convert failed: %s", err)
			return nil, err
		}

		return v2e, nil
	}, nil
}
