package v2

import (
	"math/rand"

	"code.cloudfoundry.org/go-pubsub"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
)

// PubSub provides a means for callers to subscribe to envelope streams.
type PubSub struct {
	pubsub *pubsub.PubSub
	rand   func(int64) int64
}

// NewPubSub is the constructor for PubSub.
func NewPubSub(opts ...PubSubOption) *PubSub {
	p := &PubSub{
		rand: rand.Int63n,
	}

	for _, o := range opts {
		o(p)
	}

	p.pubsub = pubsub.New(pubsub.WithRand(p.rand))

	return p
}

// PubSubOption allows for configuration of the PubSub.
type PubSubOption func(*PubSub)

// WithRand allows for configuration of the random number generator.
func WithRand(int63n func(int64) int64) PubSubOption {
	return func(r *PubSub) {
		r.rand = int63n
	}
}

// Publish writes an envelope to a subscriber. When there are multiple
// subscribers with the same shard ID, Publish will publish the envelope to
// only one of those subscribers.
func (p *PubSub) Publish(e *loggregator_v2.Envelope) {
	p.pubsub.Publish(e, noopTraverser)
}

// Subscribe associates a request with a data setter which will be invoked by
// future calls to Publish. A caller should invoke the returned function to
// unsubscribe.
func (p *PubSub) Subscribe(
	req *loggregator_v2.EgressBatchRequest,
	setter DataSetter,
) (unsubscribe func()) {

	return p.pubsub.Subscribe(subscription{setter}.set,
		pubsub.WithShardID(req.ShardId),
	)
}

type subscription struct {
	DataSetter
}

func (s subscription) set(data interface{}) {
	s.DataSetter.Set(data.(*loggregator_v2.Envelope))
}

func noopTraverser(interface{}) pubsub.Paths {
	return func(int, interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		return 0, nil, false
	}
}
