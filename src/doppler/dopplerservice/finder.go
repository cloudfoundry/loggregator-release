package dopplerservice

import (
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
	"tls",
}

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

	events chan Event
	logger *gosteno.Logger
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
	case storeadapter.CreateEvent, storeadapter.UpdateEvent:
		logging.Debugf(f.logger, "Received create/update event: %+v", event.Type)
		f.saveEndpoints(f.parseNode(*event.Node, parser))
	case storeadapter.DeleteEvent, storeadapter.ExpireEvent:
		logging.Debugf(f.logger, "Received delete/expire event: %+v", event.Type)
		f.deleteEndpoints(f.parseNode(*event.PrevNode, parser))
	}
	f.sendEvent()
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

func (f *Finder) chooseMetaAddrs() map[string]string {
	chosen := make(map[string]string)
	f.metaLock.RLock()
	defer f.metaLock.RUnlock()
	for doppler, addrs := range f.metaEndpoints {
		var chosenAddr string
		for _, addr := range addrs {
			if !supported(addr) {
				continue
			}
			chosenAddr = addr
			if strings.HasPrefix(chosenAddr, f.preferredProtocol) {
				break
			}
		}
		if chosenAddr != "" {
			chosen[doppler] = chosenAddr
		}
	}
	return chosen
}

func (f *Finder) addLegacyAddrs(addrs map[string]string) {
	f.legacyLock.RLock()
	defer f.legacyLock.RUnlock()
	for doppler, addr := range f.legacyEndpoints {
		chosenAddr, ok := addrs[doppler]
		if ok {
			if f.preferredProtocol != "udp" || strings.HasPrefix(chosenAddr, "udp") {
				continue
			}
		}
		addrs[doppler] = addr
	}
}

func (f *Finder) chooseAddrs() map[string]string {
	chosen := f.chooseMetaAddrs()
	f.addLegacyAddrs(chosen)
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

func supported(addr string) bool {
	for _, protocol := range supportedProtocols {
		if strings.HasPrefix(addr, protocol) {
			return true
		}
	}
	return false
}
