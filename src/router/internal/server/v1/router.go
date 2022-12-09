package v1

import (
	"math/rand"
	"sync"

	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"github.com/cloudfoundry/sonde-go/events"
	"google.golang.org/protobuf/proto"
)

type shardID string

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

// Router routes envelopes to particular buffers (called DataSetter here). In
// effect, the Router implements pub-sub. After a buffer has been registered
// with the Register method, calls to SendTo will ensure a particular envelope
// is sent to all registered buffers.
type Router struct {
	lock          sync.RWMutex
	subscriptions map[filter]map[shardID][]DataSetter
}

// NewRouter is the constructor for Router.
func NewRouter() *Router {
	return &Router{
		subscriptions: make(map[filter]map[shardID][]DataSetter),
	}
}

// Register stores a request with its corresponding DataSetter. Callers should
// invoke the cleanup function once a registered request should no longer
// receive envelopes.
func (r *Router) Register(req *plumbing.SubscriptionRequest, dataSetter DataSetter) (cleanup func()) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.registerSetter(req, dataSetter)

	return r.buildCleanup(req, dataSetter)
}

// SendTo sends an envelope for an application to all registered DataSetters.
func (r *Router) SendTo(appID string, envelope *events.Envelope) {
	data := r.marshal(envelope)

	if data == nil {
		return
	}

	typedFilters := r.createTypedFilters(appID, envelope)

	r.lock.RLock()
	defer r.lock.RUnlock()

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

	setters[rand.Intn(len(setters))].Set(data) //nolint:gosec
}

func (r *Router) createTypedFilters(appID string, envelope *events.Envelope) []filter {
	filters := make([]filter, 2, 4)
	filters[0] = filter{appID: "", envelopeType: r.filterTypeFromEnvelope(envelope)}
	filters[1] = filter{}

	if appID != "" {
		filters = append(filters, filter{appID: appID, envelopeType: noType})
		filters = append(filters, filter{appID: appID, envelopeType: r.filterTypeFromEnvelope(envelope)})
	}

	return filters
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
	data, err := proto.Marshal(envelope)
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
