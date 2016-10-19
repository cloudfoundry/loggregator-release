package grpcmanager

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

type Router struct {
	lock          sync.RWMutex
	streams       map[string][]DataSetter
	firehoses     map[string][]DataSetter
	firehoseConns int32
	streamConns   int32
	done          chan struct{}
}

func NewRouter() *Router {
	router := &Router{
		streams:   make(map[string][]DataSetter),
		firehoses: make(map[string][]DataSetter),
		done:      make(chan struct{}),
	}
	go router.run()
	return router
}

func (r *Router) Register(ID string, isFirehose bool, dataSetter DataSetter) (cleanup func()) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.registerSetter(ID, isFirehose, dataSetter)

	if isFirehose {
		return r.buildCleanup(ID, isFirehose, dataSetter, r.firehoses)
	}

	return r.buildCleanup(ID, isFirehose, dataSetter, r.streams)
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

func (r *Router) Stop() {
	close(r.done)
}

func (r *Router) run() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		select {
		case <-r.done:
			return
		default:
		}
		metrics.SendValue("grpcManager.numberOfFirehoseConns", float64(atomic.LoadInt32(&r.firehoseConns)), "connections")
		metrics.SendValue("grpcManager.numberOfStreamConns", float64(atomic.LoadInt32(&r.streamConns)), "connections")
	}
}

func (r *Router) registerSetter(ID string, isFirehose bool, dataSetter DataSetter) {
	if isFirehose {
		atomic.AddInt32(&r.firehoseConns, 1)
		r.firehoses[ID] = append(r.firehoses[ID], dataSetter)
		return
	}

	atomic.AddInt32(&r.streamConns, 1)
	r.streams[ID] = append(r.streams[ID], dataSetter)
}

func (r *Router) buildCleanup(ID string, isFirehose bool, dataSetter DataSetter, m map[string][]DataSetter) func() {
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

		if isFirehose {
			atomic.AddInt32(&r.firehoseConns, -1)
			return
		}

		atomic.AddInt32(&r.streamConns, -1)
	}
}

func (r *Router) marshal(envelope *events.Envelope) []byte {
	data, err := envelope.Marshal()
	if err != nil {
		return nil
	}

	return data
}
