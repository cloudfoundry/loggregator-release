package dopplerservice_test

import "github.com/cloudfoundry/storeadapter"

type mockStoreAdapter struct {
	WatchCalled chan bool
	WatchInput  struct {
		key chan string
	}
	WatchOutput struct {
		events chan (<-chan storeadapter.WatchEvent)
		stop   chan (chan<- bool)
		errors chan (<-chan error)
	}
	ListRecursivelyCalled chan bool
	ListRecursivelyInput  struct {
		key chan string
	}
	ListRecursivelyOutput struct {
		ret0 chan storeadapter.StoreNode
		ret1 chan error
	}
}

func newMockStoreAdapter() *mockStoreAdapter {
	m := &mockStoreAdapter{}
	m.WatchCalled = make(chan bool, 100)
	m.WatchInput.key = make(chan string, 100)
	m.WatchOutput.events = make(chan (<-chan storeadapter.WatchEvent), 100)
	m.WatchOutput.stop = make(chan (chan<- bool), 100)
	m.WatchOutput.errors = make(chan (<-chan error), 100)
	m.ListRecursivelyCalled = make(chan bool, 100)
	m.ListRecursivelyInput.key = make(chan string, 100)
	m.ListRecursivelyOutput.ret0 = make(chan storeadapter.StoreNode, 100)
	m.ListRecursivelyOutput.ret1 = make(chan error, 100)
	return m
}
func (m *mockStoreAdapter) Watch(key string) (events <-chan storeadapter.WatchEvent, stop chan<- bool, errors <-chan error) {
	m.WatchCalled <- true
	m.WatchInput.key <- key
	return <-m.WatchOutput.events, <-m.WatchOutput.stop, <-m.WatchOutput.errors
}
func (m *mockStoreAdapter) ListRecursively(key string) (storeadapter.StoreNode, error) {
	m.ListRecursivelyCalled <- true
	m.ListRecursivelyInput.key <- key
	return <-m.ListRecursivelyOutput.ret0, <-m.ListRecursivelyOutput.ret1
}
