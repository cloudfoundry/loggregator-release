package clientpool

import (
	"doppler/dopplerservice"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Eventer interface {
	Next() dopplerservice.Event
}

type DopplerFinder struct {
	eventer Eventer

	cond           *sync.Cond
	dopplers       []string
	repeatedEvents chan dopplerservice.Event
}

func NewDopplerFinder(eventer Eventer) *DopplerFinder {
	f := &DopplerFinder{
		eventer:        eventer,
		cond:           sync.NewCond(new(sync.Mutex)),
		repeatedEvents: make(chan dopplerservice.Event),
	}
	go f.run()
	return f
}

func (f *DopplerFinder) Doppler() string {
	for {
		f.cond.L.Lock()

		if len(f.dopplers) == 0 {
			f.cond.Wait()
			f.cond.L.Unlock()
			continue
		}

		defer f.cond.L.Unlock()
		return f.dopplers[rand.Intn(len(f.dopplers))]
	}
}

// TODO: Delete this once UDP is deprecated
func (f *DopplerFinder) Next() dopplerservice.Event {
	return <-f.repeatedEvents
}

func (f *DopplerFinder) run() {
	for {
		event := f.eventer.Next()
		f.repeatEvent(event)

		log.Printf("Available Dopplers: %v", event.GRPCDopplers)
		f.cond.L.Lock()
		f.dopplers = event.GRPCDopplers
		f.cond.Broadcast()
		f.cond.L.Unlock()
	}
}

func (f *DopplerFinder) repeatEvent(event dopplerservice.Event) {
	select {
	case f.repeatedEvents <- event:
	case <-time.After(time.Second):
		log.Panic("The repeated events are not being consumed!")
	}
}
