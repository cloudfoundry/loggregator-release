package dopplerservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/logging"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
)

var supportedProtocols = []string{
	"udp",
	"tcp",
	"tls",
}

//go:generate hel --type StoreAdapter --output mock_store_adapter_test.go

type StoreAdapter interface {
	Watch(key string) (events <-chan storeadapter.WatchEvent, stop chan<- bool, errors <-chan error)
	ListRecursively(key string) (storeadapter.StoreNode, error)
}

type set map[string]struct{}

func (s set) add(value string) {
	s[value] = struct{}{}
}

func (s set) contains(value string) bool {
	_, ok := s[value]
	return ok
}

func (s *set) UnmarshalJSON(value []byte) error {
	m := make(set)
	var src []string
	if err := json.Unmarshal(value, &src); err != nil {
		return err
	}
	for _, v := range src {
		m[v] = struct{}{}
	}
	*s = m
	return nil
}

type DopplerEvent struct {
	Version   int `json:"version"`
	Endpoints set `json:"endpoints"`
}

type Event struct {
	UDPDopplers []string
	TLSDopplers []string
	TCPDopplers []string
}

func (e Event) empty() bool {
	return len(e.UDPDopplers) == 0 &&
		len(e.TCPDopplers) == 0 &&
		len(e.TLSDopplers) == 0
}

type Finder struct {
	adapter              StoreAdapter
	legacyPort           int
	protocols            []string
	preferredDopplerZone string
	metaEndpoints        map[string][]string
	metaLock             sync.RWMutex
	legacyEndpoints      map[string]string
	legacyLock           sync.RWMutex

	events chan Event
	logger *gosteno.Logger
}

func NewFinder(adapter StoreAdapter, legacyPort int, protocols []string, preferredDopplerZone string, logger *gosteno.Logger) *Finder {
	return &Finder{
		metaEndpoints:        make(map[string][]string),
		legacyEndpoints:      make(map[string]string),
		adapter:              adapter,
		protocols:            protocols,
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

func (f *Finder) Start() {
	events, stop, errs := f.adapter.Watch(META_ROOT)
	f.read(META_ROOT, f.parseMetaValue)

	go f.run(META_ROOT, events, errs, stop, f.parseMetaValue)

	events, stop, errs = f.adapter.Watch(LEGACY_ROOT)
	f.read(LEGACY_ROOT, f.parseLegacyValue)

	go f.run(LEGACY_ROOT, events, errs, stop, f.parseLegacyValue)

	if !f.hasAddrs() {
		return
	}
	f.sendEvent()
}

func (f *Finder) Next() Event {
	return <-f.events
}

func (f *Finder) hasAddrs() bool {
	f.legacyLock.Lock()
	f.metaLock.Lock()
	defer f.legacyLock.Unlock()
	defer f.metaLock.Unlock()
	return len(f.legacyEndpoints) > 0 || len(f.metaEndpoints) > 0
}

func (f *Finder) handleEvent(event *storeadapter.WatchEvent, parser func([]byte) (interface{}, error)) {
	switch event.Type {
	case storeadapter.InvalidEvent:
		f.logger.Errorf("Received invalid event: %+v", event)
		return
	case storeadapter.UpdateEvent:
		if f.equal(event.PrevNode, event.Node) {
			return
		}
		fallthrough
	case storeadapter.CreateEvent:
		logging.Debugf(f.logger, "Received create/update event: %+v", event.Type)
		f.saveEndpoints(f.parseNode(*event.Node, parser))
	case storeadapter.DeleteEvent, storeadapter.ExpireEvent:
		logging.Debugf(f.logger, "Received delete/expire event: %+v", event.Type)
		f.deleteEndpoints(f.parseNode(*event.PrevNode, parser))
	}
	f.sendEvent()
}

func (f *Finder) equalLegacy(prev, next *storeadapter.StoreNode) bool {
	return bytes.Equal(prev.Value, next.Value)
}

func (f *Finder) equal(prev, next *storeadapter.StoreNode) bool {
	if strings.HasPrefix(prev.Key, LEGACY_ROOT) {
		return f.equalLegacy(prev, next)
	}

	var prevEvent, nextEvent DopplerEvent
	err := json.Unmarshal(prev.Value, &prevEvent)
	if err != nil {
		f.logger.Errorf("Unmarshaling: %v", err)
		return false
	}
	err = json.Unmarshal(next.Value, &nextEvent)
	if err != nil {
		f.logger.Errorf("Unmarshaling: %v", err)
		return false
	}
	if len(prevEvent.Endpoints) != len(nextEvent.Endpoints) {
		return false
	}
	for v := range prevEvent.Endpoints {
		if !nextEvent.Endpoints.contains(v) {
			return false
		}
	}
	return true
}

func (f *Finder) run(etcdRoot string, events <-chan storeadapter.WatchEvent, errs <-chan error, stop chan<- bool, parser func([]byte) (interface{}, error)) {
	for {
		select {
		case event := <-events:
			f.handleEvent(&event, parser)
		case err := <-errs:
			f.logger.Errord(map[string]interface{}{
				"error": err.Error()}, "Finder: Watch sent an error")
			time.Sleep(time.Second)
			events, stop, errs = f.adapter.Watch(etcdRoot)
			f.read(etcdRoot, parser)
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

func (f *Finder) sendEvent() {
	event := f.eventWithPrefix(f.preferredDopplerZone)
	if event.empty() {
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
		case strings.HasPrefix(addr, "udp"):
			event.UDPDopplers = append(event.UDPDopplers, strings.TrimPrefix(addr, "udp://"))
		case strings.HasPrefix(addr, "tcp"):
			event.TCPDopplers = append(event.TCPDopplers, strings.TrimPrefix(addr, "tcp://"))
		case strings.HasPrefix(addr, "tls"):
			event.TLSDopplers = append(event.TLSDopplers, strings.TrimPrefix(addr, "tls://"))
		default:
			f.logger.Errorf("Unexpected address for doppler %s (invalid protocol): %s", doppler, addr)
		}
	}

	return event
}

func (f *Finder) chooseAddrs() map[string]string {
	chosen := f.chooseMetaAddrs()
	f.addLegacyAddrs(chosen)
	return chosen
}

func (f *Finder) chooseMetaAddrs() map[string]string {
	chosen := make(map[string]string)
	f.metaLock.RLock()
	defer f.metaLock.RUnlock()
	for doppler, addrs := range f.metaEndpoints {
		// Set currentIndex to one greater than any possible index of f.protocols
		var currentIndex int = len(f.protocols)
		for _, addr := range addrs {
			if addr == "" || !supported(addr) {
				continue
			}
			index, err := f.protocolIndex(addr)
			if err == nil && index < currentIndex {
				currentIndex = index
				chosen[doppler] = addr
			}
		}
	}
	return chosen
}

func (f *Finder) addLegacyAddrs(addrs map[string]string) {
	f.legacyLock.RLock()
	defer f.legacyLock.RUnlock()
	udpIdx, err := f.protocolIndex("udp")
	if err != nil {
		return
	}
	for doppler, legacyAddr := range f.legacyEndpoints {
		chosenAddr, ok := addrs[doppler]
		if ok {
			idx, err := f.protocolIndex(chosenAddr)
			if err != nil {
				// If chosen addr is not supported (should never happen)
				msg := fmt.Sprintf("Somehow chosenAddr %s (for doppler %s) is "+
					"not in this finder's protocol list %v",
					chosenAddr,
					doppler,
					f.protocols,
				)
				panic(msg)
			}
			if strings.HasPrefix(chosenAddr, "udp") || idx < udpIdx {
				// We already have an address we prefer over the legacy address
				continue
			}
		}
		addrs[doppler] = legacyAddr
	}
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

func (f *Finder) protocolIndex(addr string) (int, error) {
	for i, protocol := range f.protocols {
		if strings.HasPrefix(addr, protocol) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("Protocol %s is not present", addr)
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

func supported(addr string) bool {
	for _, protocol := range supportedProtocols {
		if strings.HasPrefix(addr, protocol) {
			return true
		}
	}
	return false
}
