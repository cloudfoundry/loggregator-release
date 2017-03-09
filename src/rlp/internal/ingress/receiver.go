package ingress

import (
	"log"
	"plumbing"
	v2 "plumbing/v2"

	"golang.org/x/net/context"
)

type Subscriber interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (recv func() ([]byte, error), err error)
}

type Converter interface {
	Convert(data []byte) (envelope *v2.Envelope, err error)
}

type RequestConverter interface {
	Convert(req *v2.EgressRequest) *plumbing.SubscriptionRequest
}

type Receiver struct {
	converter    Converter
	reqConverter RequestConverter
	subscriber   Subscriber
}

func NewReceiver(c Converter, r RequestConverter, s Subscriber) *Receiver {
	return &Receiver{
		converter:    c,
		reqConverter: r,
		subscriber:   s,
	}
}

func (r *Receiver) Subscribe(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error) {
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

		v2e, err := r.converter.Convert(data)
		if err != nil {
			log.Printf("V1->V2 convert failed: %s", err)
			return nil, err
		}

		return v2e, nil
	}, nil
}
