package dopplerservice

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"net/url"
)

//go:generate hel --type StoreAdapter --output mock_store_adapter_test.go

type StoreAdapter interface {
	Watch(key string) (events <-chan storeadapter.WatchEvent, stop chan<- bool, errors <-chan error)
	ListRecursively(key string) (storeadapter.StoreNode, error)
}

type Event struct {
	UDPDopplers []string
	TLSDopplers []string
}

type Finder struct {
	adapter              StoreAdapter
	legacyPort           int
	preferredProtocol    string
	preferredDopplerZone string
	metaEndpoints        map[string][]string
	metaLock             sync.RWMutex
	legacyEndpoints      map[string]string
	legacyLock           sync.RWMutex

	events               chan Event
	logger               *gosteno.Logger
}

func NewFinder(adapter StoreAdapter, legacyPort int, preferredProtocol string, preferredDopplerZone string, logger *gosteno.Logger) *Finder {
	return &Finder{
		metaEndpoints:        make(map[string][]string),
		legacyEndpoints:      make(map[string]string),
		adapter:              adapter,
		preferredProtocol:    preferredProtocol,
		preferredDopplerZone: preferredDopplerZone,
		legacyPort:           legacyPort,
		events:               make(chan Event, 10),
		logger:               logger,
	}
}

func (f *Finder) WebsocketServers() []string {
	f.legacyLock.RLock()
	defer f.legacyLock.RUnlock()
	wsServers := make([]string, 0, len(f.legacyEndpoints))
	for _, server := range f.legacyEndpoints {
		serverURL, err := url.Parse(server)
		if err == nil {
			wsServers = append(wsServers, serverURL.Host)
		}
	}
	return wsServers
}

func (f *Finder) Start() error {
	events, stop, errs := f.adapter.Watch(META_ROOT)
	if err := f.read(META_ROOT, f.parseMetaValue); err != nil {
		return err
	}
	go f.run(events, errs, stop, f.parseMetaValue)

	events, stop, errs = f.adapter.Watch(LEGACY_ROOT)
	if err := f.read(LEGACY_ROOT, f.parseLegacyValue); err != nil {
		return err
	}
	go f.run(events, errs, stop, f.parseLegacyValue)

	f.sendEvent()
	return nil
}

func (f *Finder) handleEvent(event *storeadapter.WatchEvent, parser func([]byte) (interface{}, error)) {
	switch event.Type {
	case storeadapter.InvalidEvent:
		f.logger.Errorf("Received invalid event: %+v", event)
		return
	case storeadapter.CreateEvent, storeadapter.UpdateEvent:
		f.saveEndpoints(f.parseNode(*event.Node, parser))
	case storeadapter.DeleteEvent, storeadapter.ExpireEvent:
		f.deleteEndpoints(f.parseNode(*event.PrevNode, parser))
	}
	f.sendEvent()
}

func (f *Finder) run(events <-chan storeadapter.WatchEvent, errs <-chan error, stop chan<- bool, parser func([]byte) (interface{}, error)) {
	for {
		select {
		case event := <-events:
			f.handleEvent(&event, parser)
		case err := <-errs:
			f.logger.Errord(map[string]interface{}{
				"error": err.Error()}, "Finder: Watch failed")
		}
	}
}

func (f *Finder) read(root string, parser func([]byte) (interface{}, error)) error {
	node, err := f.adapter.ListRecursively(root)
	if err != nil {
		return err
	}
	for _, node := range findLeafs(node) {
		doppler, endpoints := f.parseNode(node, parser)
		f.saveEndpoints(doppler, endpoints)
	}
	return nil
}

func (f *Finder) deleteEndpoints(key string, value interface{}) {
	switch value.(type) {
	case []string:
		f.metaLock.Lock()
		defer f.metaLock.Unlock()
		delete(f.metaEndpoints, key)
	case string:
		f.legacyLock.Lock()
		defer f.legacyLock.Unlock()
		delete(f.legacyEndpoints, key)
	}
}

func (f *Finder) saveEndpoints(key string, value interface{}) {
	switch src := value.(type) {
	case []string:
		f.metaLock.Lock()
		defer f.metaLock.Unlock()
		f.metaEndpoints[key] = src
	case string:
		f.legacyLock.Lock()
		defer f.legacyLock.Unlock()
		f.legacyEndpoints[key] = src
	}
}

func (f *Finder) parseNode(node storeadapter.StoreNode, parser func([]byte) (interface{}, error)) (dopplerKey string, value interface{}) {
	nodeValue, err := parser(node.Value)
	if err != nil {
		f.logger.Errorf("could not parse etcd node %s: %v", string(node.Value), err)
		return "", nil
	}

	switch nodeValue.(type) {
	case []string:
		dopplerKey = strings.TrimPrefix(node.Key, META_ROOT)
	case string:
		dopplerKey = strings.TrimPrefix(node.Key, LEGACY_ROOT)
	}
	dopplerKey = strings.TrimPrefix(dopplerKey, "/")
	return dopplerKey, nodeValue
}

func (f *Finder) Next() Event {
	return <-f.events
}

func (f *Finder) sendEvent() {
	event := f.eventWithPrefix(f.preferredDopplerZone)
	if len(event.TLSDopplers) == 0 && len(event.UDPDopplers) == 0 {
		event = f.eventWithPrefix("")
	}

	f.events <- event
}

func (f *Finder) eventWithPrefix(dopplerPrefix string) Event {
	chosenAddrs := f.chooseAddrs()

	var event Event
	for doppler, addr := range chosenAddrs {
		if !strings.HasPrefix(doppler, dopplerPrefix) {
			continue
		}
		switch {
		case strings.HasPrefix(addr, "tls"):
			event.TLSDopplers = append(event.TLSDopplers, strings.TrimPrefix(addr, "tls://"))
		case strings.HasPrefix(addr, "udp"):
			event.UDPDopplers = append(event.UDPDopplers, strings.TrimPrefix(addr, "udp://"))
		default:
			f.logger.Errorf("Unexpected address for doppler %s (invalid protocol): %s", doppler, addr)
		}
	}
	return event
}

func (f *Finder) chooseAddrs() map[string]string {
	chosen := make(map[string]string)

	//TODO: Explicitly set default UDP address as opposed to the last one
	// TODO: Break up into separate functions
	func() {
		f.metaLock.RLock()
		defer f.metaLock.RUnlock()
		for doppler, addrs := range f.metaEndpoints {
			var addr string
			for _, addr = range addrs {
				if strings.HasPrefix(addr, f.preferredProtocol) {
					break
				}
			}
			chosen[doppler] = addr
		}
	}()

	func() {
		f.legacyLock.RLock()
		defer f.legacyLock.RUnlock()
		for doppler, addr := range f.legacyEndpoints {
			if _, ok := chosen[doppler]; ok && f.preferredProtocol != "udp" {
				continue
			}
			chosen[doppler] = addr
		}
	}()
	return chosen
}

func (f *Finder) parseMetaValue(leafValue []byte) (interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(leafValue, &data); err != nil {
		return nil, err
	}

	// json.Unmarshal always unmarshals to a []interface{}, so copy to a []string.
	endpointSlice := data["endpoints"].([]interface{})
	endpoints := make([]string, 0, len(endpointSlice))
	for _, endpoint := range endpointSlice {
		endpoints = append(endpoints, endpoint.(string))
	}
	return endpoints, nil
}

func (f *Finder) parseLegacyValue(leafValue []byte) (interface{}, error) {
	return fmt.Sprintf("udp://%s:%d", string(leafValue), f.legacyPort), nil
}

// leafNodes is here to make it invisible to us how deeply nested our values are.
func findLeafs(root storeadapter.StoreNode) []storeadapter.StoreNode {
	if !root.Dir {
		if len(root.Value) == 0 {
			return []storeadapter.StoreNode{}
		}
		return []storeadapter.StoreNode{root}
	}

	leaves := []storeadapter.StoreNode{}
	for _, node := range root.ChildNodes {
		leaves = append(leaves, findLeafs(node)...)
	}
	return leaves
}
