package plumbing

import "code.cloudfoundry.org/loggregator/dopplerservice"

type StaticFinder struct {
	event chan dopplerservice.Event
}

func NewStaticFinder(addrs []string) *StaticFinder {
	event := make(chan dopplerservice.Event, 10)
	event <- dopplerservice.Event{
		GRPCDopplers: addrs,
	}
	return &StaticFinder{
		event: event,
	}
}

func (f *StaticFinder) Start() {}

func (f *StaticFinder) Stop() {
	f.event <- dopplerservice.Event{
		GRPCDopplers: []string{},
	}
}

func (f *StaticFinder) Next() dopplerservice.Event {
	return <-f.event
}
