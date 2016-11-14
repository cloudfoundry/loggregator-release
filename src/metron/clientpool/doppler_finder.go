package clientpool

import (
	"doppler/dopplerservice"
	"log"
	"math/rand"
	"sync"
)

type Eventer interface {
	Next() dopplerservice.Event
}

type DopplerFinder struct {
	eventer Eventer

	cond     *sync.Cond
	dopplers []string
}

func NewDopplerFinder(eventer Eventer) *DopplerFinder {
	f := &DopplerFinder{
		eventer: eventer,
		cond:    sync.NewCond(new(sync.Mutex)),
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

func (f *DopplerFinder) run() {
	for {
		event := f.eventer.Next()
		log.Printf("Available Dopplers: %v", event.UDPDopplers)

		f.cond.L.Lock()
		f.dopplers = event.UDPDopplers
		f.cond.Broadcast()
		f.cond.L.Unlock()
	}
}
