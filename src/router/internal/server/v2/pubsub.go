package v2

import (
	"hash/crc64"
	"math/rand"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-pubsub"
	"code.cloudfoundry.org/go-pubsub/pubsub-gen/setters"
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

	p.pubsub = pubsub.New(
		pubsub.WithRand(p.rand),
		pubsub.WithDeterministicHashing(func(i interface{}) uint64 {
			return p.hashEnvelope(i.(*loggregator_v2.Envelope))
		}),
	)

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
	var unsubscribes []func()
	for _, s := range req.GetSelectors() {
		// Selector.Message is required.
		if s.Message == nil {
			continue
		}

		unsubscribes = append(unsubscribes, p.pubsub.Subscribe(
			subscription(s.GetSourceId(), setter),
			pubsub.WithShardID(req.GetShardId()),
			pubsub.WithDeterministicRouting(req.GetDeterministicName()),
			pubsub.WithPath(envelopeTraverserCreatePath(buildFilter(s))),
		))
	}

	return func() {
		for _, u := range unsubscribes {
			u()
		}
	}
}

func (p *PubSub) hashEnvelope(e *loggregator_v2.Envelope) uint64 {
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Counter:
		return crc64.Checksum([]byte(e.GetCounter().GetName()), tableECMA)
	case *loggregator_v2.Envelope_Gauge:
		var h uint64
		for name := range e.GetGauge().GetMetrics() {
			h += crc64.Checksum([]byte(name), tableECMA)
		}
		return h
	default:
		return rand.Uint64()
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

	switch x := s.Message.(type) {
	case *loggregator_v2.Selector_Log:
		f.Message_Envelope_Log = &Envelope_LogFilter{}
	case *loggregator_v2.Selector_Counter:
		filter := &Envelope_CounterFilter{}
		if x.Counter.GetName() != "" {
			filter.Counter = &CounterFilter{
				Name: setters.String(x.Counter.GetName()),
			}
		}
		f.Message_Envelope_Counter = filter
	case *loggregator_v2.Selector_Gauge:
		filter := &Envelope_GaugeFilter{}

		if len(x.Gauge.GetNames()) != 0 {
			filter.Gauge = &GaugeFilter{
				Metrics: x.Gauge.GetNames(),
			}
		}

		f.Message_Envelope_Gauge = filter
	case *loggregator_v2.Selector_Timer:
		f.Message_Envelope_Timer = &Envelope_TimerFilter{}
	case *loggregator_v2.Selector_Event:
		f.Message_Envelope_Event = &Envelope_EventFilter{}
	}

	return f
}
