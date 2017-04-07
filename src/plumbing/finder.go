package plumbing

import "dopplerservice"

type StaticFinder struct {
	event chan dopplerservice.Event
}

func NewStaticFinder(addrs []string) *StaticFinder {
	event := make(chan dopplerservice.Event, 1)
	event <- dopplerservice.Event{
		GRPCDopplers: addrs,
	}
	return &StaticFinder{
		event: event,
	}
}

func (f *StaticFinder) Start() {}

func (f *StaticFinder) Next() dopplerservice.Event {
	return <-f.event
}
