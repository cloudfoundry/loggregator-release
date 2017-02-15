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

type Receiver struct {
	converter  Converter
	subscriber Subscriber
}

func NewReceiver(c Converter, s Subscriber) *Receiver {
	return &Receiver{
		converter:  c,
		subscriber: s,
	}
}

func (r *Receiver) Subscribe(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error) {
	v1Rx, err := r.subscriber.Subscribe(ctx, convertReq(req))
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

func convertReq(v2req *v2.EgressRequest) *plumbing.SubscriptionRequest {
	return &plumbing.SubscriptionRequest{
		ShardID: v2req.ShardId,
		Filter:  convertFilter(v2req.GetFilter()),
	}
}

func convertFilter(v2filter *v2.Filter) *plumbing.Filter {
	if v2filter == nil {
		return nil
	}
	return &plumbing.Filter{
		AppID: v2filter.SourceId,
	}
}
