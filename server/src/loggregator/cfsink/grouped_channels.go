package cfsink

import (
	"strings"
	"sync"
)

func newGroupedChannels() *groupedChannels {
	return &groupedChannels{make(map[string]map[chan []byte]bool), new(sync.Mutex)}
}

type groupedChannels struct {
	channels map[string]map[chan []byte]bool //{key => {channel => true, ...}, ...}
	mutex    *sync.Mutex
}

func keyify(keys []string) string {
	return strings.Join(keys, ":")
}

func (gc *groupedChannels) add(channel chan []byte, keys ...string) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	if gc.channels[keyify(keys)] == nil {
		gc.channels[keyify(keys)] = make(map[chan []byte]bool)
	}
	gc.channels[keyify(keys)][channel] = true

}

func (gc *groupedChannels) get(keys ...string) []chan []byte {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	result := make([]chan []byte, 0, len(gc.channels[keyify(keys)]))

	for channel, _ := range gc.channels[keyify(keys)] {
		result = append(result, channel)
	}

	return result
}

func (gc *groupedChannels) delete(channel chan []byte, keys ...string) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	delete(gc.channels[keyify(keys)], channel)
}
