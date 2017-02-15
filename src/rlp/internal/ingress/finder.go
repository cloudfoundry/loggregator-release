package ingress

import "doppler/dopplerservice"

type Finder struct {
	event chan dopplerservice.Event
}

func NewFinder(addrs []string) *Finder {
	event := make(chan dopplerservice.Event, 1)
	event <- dopplerservice.Event{
		GRPCDopplers: addrs,
	}
	return &Finder{
		event: event,
	}
}

func (f *Finder) Next() dopplerservice.Event {
	return <-f.event
}
