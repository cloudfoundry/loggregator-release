package v1

import (
	"math/rand"
	"plumbing"
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
)

type shardID string

type Router struct {
	lock          sync.RWMutex
	subscriptions map[plumbing.Filter]map[shardID][]DataSetter
}

func NewRouter() *Router {
	return &Router{
		subscriptions: make(map[plumbing.Filter]map[shardID][]DataSetter),
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

	filter := plumbing.Filter{
		AppID: appID,
	}

	for id, setters := range r.subscriptions[filter] {
		r.writeToShard(id, setters, data)
	}

	var noFilter plumbing.Filter
	for id, setters := range r.subscriptions[noFilter] {
		r.writeToShard(id, setters, data)
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

func (r *Router) registerSetter(req *plumbing.SubscriptionRequest, dataSetter DataSetter) {
	var filter plumbing.Filter
	if req.Filter != nil {
		filter = *req.Filter
	}

	m, ok := r.subscriptions[filter]
	if !ok {
		m = make(map[shardID][]DataSetter)
		r.subscriptions[filter] = m
	}

	m[shardID(req.ShardID)] = append(m[shardID(req.ShardID)], dataSetter)
}

func (r *Router) buildCleanup(req *plumbing.SubscriptionRequest, dataSetter DataSetter) func() {
	return func() {
		r.lock.Lock()
		defer r.lock.Unlock()

		var filter plumbing.Filter
		if req.Filter != nil {
			filter = *req.Filter
		}

		var setters []DataSetter
		for _, s := range r.subscriptions[filter][shardID(req.ShardID)] {
			if s != dataSetter {
				setters = append(setters, s)
			}
		}

		if len(setters) > 0 {
			r.subscriptions[filter][shardID(req.ShardID)] = setters
			return
		}

		delete(r.subscriptions[filter], shardID(req.ShardID))

		if len(r.subscriptions[filter]) == 0 {
			delete(r.subscriptions, filter)
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
