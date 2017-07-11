package v1

import (
	"math/rand"
	"sync"

	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/cloudfoundry/sonde-go/events"
)

type shardID string

type Router struct {
	lock          sync.RWMutex
	subscriptions map[filter]map[shardID][]DataSetter
}

type filterType uint8

const (
	noType filterType = iota
	logType
	metricType
)

type filter struct {
	appID        string
	envelopeType filterType
}

func NewRouter() *Router {
	return &Router{
		subscriptions: make(map[filter]map[shardID][]DataSetter),
	}
}

func (r *Router) Register(req *plumbing.SubscriptionRequest, dataSetter DataSetter) (cleanup func()) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.registerSetter(req, dataSetter)

	return r.buildCleanup(req, dataSetter)
}

func (r *Router) SendTo(appID string, envelope *events.Envelope) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	data := r.marshal(envelope)

	if data == nil {
		return
	}

	typedFilters := r.createTypedFilters(appID, envelope)
	for _, typedFilter := range typedFilters {
		for id, setters := range r.subscriptions[typedFilter] {
			r.writeToShard(id, setters, data)
		}
	}
}

func (r *Router) writeToShard(id shardID, setters []DataSetter, data []byte) {
	if id == "" {
		for _, setter := range setters {
			setter.Set(data)
		}
		return
	}

	setters[rand.Intn(len(setters))].Set(data)
}

func (r *Router) createTypedFilters(appID string, envelope *events.Envelope) []filter {
	return []filter{
		{appID: appID, envelopeType: noType},
		{appID: appID, envelopeType: r.filterTypeFromEnvelope(envelope)},
		{appID: "", envelopeType: r.filterTypeFromEnvelope(envelope)},
		{},
	}
}

func (r *Router) registerSetter(req *plumbing.SubscriptionRequest, dataSetter DataSetter) {
	f := r.convertFilter(req)

	m, ok := r.subscriptions[f]
	if !ok {
		m = make(map[shardID][]DataSetter)
		r.subscriptions[f] = m
	}

	m[shardID(req.ShardID)] = append(m[shardID(req.ShardID)], dataSetter)
}

func (r *Router) buildCleanup(req *plumbing.SubscriptionRequest, dataSetter DataSetter) func() {
	return func() {
		r.lock.Lock()
		defer r.lock.Unlock()

		f := r.convertFilter(req)
		var setters []DataSetter
		for _, s := range r.subscriptions[f][shardID(req.ShardID)] {
			if s != dataSetter {
				setters = append(setters, s)
			}
		}

		if len(setters) > 0 {
			r.subscriptions[f][shardID(req.ShardID)] = setters
			return
		}

		delete(r.subscriptions[f], shardID(req.ShardID))

		if len(r.subscriptions[f]) == 0 {
			delete(r.subscriptions, f)
		}
	}
}

func (r *Router) marshal(envelope *events.Envelope) []byte {
	data, err := envelope.Marshal()
	if err != nil {
		return nil
	}

	return data
}

func (r *Router) convertFilter(req *plumbing.SubscriptionRequest) filter {
	if req.GetFilter() == nil {
		return filter{}
	}
	f := filter{
		appID: req.Filter.AppID,
	}
	if req.GetFilter().GetLog() != nil {
		f.envelopeType = logType
	}
	if req.GetFilter().GetMetric() != nil {
		f.envelopeType = metricType
	}
	return f
}

func (r *Router) filterTypeFromEnvelope(envelope *events.Envelope) filterType {
	switch envelope.GetEventType() {
	case events.Envelope_LogMessage:
		return logType
	default:
		return metricType
	}
}
