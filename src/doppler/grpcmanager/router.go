package grpcmanager

import (
	"math/rand"
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
)

type Router struct {
	lock      sync.RWMutex
	streams   map[string][]DataSetter
	firehoses map[string][]DataSetter
}

func NewRouter() *Router {
	return &Router{
		streams:   make(map[string][]DataSetter),
		firehoses: make(map[string][]DataSetter),
	}
}

func (r *Router) Register(ID string, isFirehose bool, dataSetter DataSetter) (cleanup func()) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.registerSetter(ID, isFirehose, dataSetter)

	if isFirehose {
		return r.buildCleanup(ID, dataSetter, r.firehoses)
	}

	return r.buildCleanup(ID, dataSetter, r.streams)
}

func (r *Router) SendTo(appID string, envelope *events.Envelope) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	data := r.marshal(envelope)
	if data == nil {
		return
	}

	for _, setter := range r.streams[appID] {
		setter.Set(data)
	}

	for _, setters := range r.firehoses {
		setters[rand.Intn(len(setters))].Set(data)
	}
}

func (r *Router) registerSetter(ID string, isFirehose bool, dataSetter DataSetter) {
	if isFirehose {
		r.firehoses[ID] = append(r.firehoses[ID], dataSetter)
		return
	}

	r.streams[ID] = append(r.streams[ID], dataSetter)
}

func (r *Router) buildCleanup(ID string, dataSetter DataSetter, m map[string][]DataSetter) func() {
	return func() {
		r.lock.Lock()
		defer r.lock.Unlock()

		var setters []DataSetter
		for _, s := range m[ID] {
			if s != dataSetter {
				setters = append(setters, s)
			}
		}

		if len(setters) > 0 {
			m[ID] = setters
			return
		}
		delete(m, ID)
	}
}

func (r *Router) marshal(envelope *events.Envelope) []byte {
	data, err := envelope.Marshal()
	if err != nil {
		return nil
	}

	return data
}
