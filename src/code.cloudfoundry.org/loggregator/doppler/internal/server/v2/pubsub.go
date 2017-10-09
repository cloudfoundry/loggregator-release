package v2

import (
	"math/rand"

	"code.cloudfoundry.org/go-pubsub"
	"code.cloudfoundry.org/go-pubsub/pubsub-gen/setters"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
)

//go:generate ./generate.sh

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
	p.pubsub.Publish(e, envelopeTraverserTraverse)
}

// Subscribe associates a request with a data setter which will be invoked by
// future calls to Publish. A caller should invoke the returned function to
// unsubscribe.
func (p *PubSub) Subscribe(
	req *loggregator_v2.EgressBatchRequest,
	setter DataSetter,
) (unsubscribe func()) {
	if len(req.GetSelectors()) < 1 {
		return p.pubsub.Subscribe(
			subscription("", setter),
			pubsub.WithShardID(req.GetShardId()),
		)
	}

	var unsubscribes []func()
	for _, s := range req.GetSelectors() {
		unsubscribes = append(unsubscribes, p.pubsub.Subscribe(
			subscription(s.GetSourceId(), setter),
			pubsub.WithShardID(req.GetShardId()),
			pubsub.WithPath(envelopeTraverserCreatePath(buildFilter(s))),
		))
	}

	return func() {
		for _, u := range unsubscribes {
			u()
		}
	}
}

func subscription(sourceID string, d DataSetter) pubsub.Subscription {
	return func(data interface{}) {
		e := data.(*loggregator_v2.Envelope)

		// This is protection against a hash collision.
		// If we knew of two strings that have the same crc64 hash, then
		// we could write a test. Until then, this block remains untested.
		if sourceID != "" && sourceID != e.GetSourceId() {
			return
		}

		d.Set(e)
	}
}

func buildFilter(s *loggregator_v2.Selector) *EnvelopeFilter {
	f := &EnvelopeFilter{}

	if s.GetSourceId() != "" {
		f.SourceId = setters.String(s.GetSourceId())
	}

	switch s.Message.(type) {
	case *loggregator_v2.Selector_Log:
		f.Message_Envelope_Log = &Envelope_LogFilter{}
	case *loggregator_v2.Selector_Counter:
		f.Message_Envelope_Counter = &Envelope_CounterFilter{}
	case *loggregator_v2.Selector_Gauge:
		f.Message_Envelope_Gauge = &Envelope_GaugeFilter{}
	case *loggregator_v2.Selector_Timer:
		f.Message_Envelope_Timer = &Envelope_TimerFilter{}
	case *loggregator_v2.Selector_Event:
		f.Message_Envelope_Event = &Envelope_EventFilter{}
	}

	return f
}
