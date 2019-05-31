package plumbing

type StaticFinder struct {
	event chan Event
}

func NewStaticFinder(addrs []string) *StaticFinder {
	event := make(chan Event, 10)
	event <- Event{
		GRPCDopplers: addrs,
	}
	return &StaticFinder{
		event: event,
	}
}

func (f *StaticFinder) Start() {}

func (f *StaticFinder) Stop() {
	f.event <- Event{
		GRPCDopplers: []string{},
	}
}

func (f *StaticFinder) Next() Event {
	return <-f.event
}
