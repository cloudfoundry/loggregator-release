package groupedchannels

import (
	"strings"
	"sync"
)

func NewGroupedChannels() *GroupedChannels {
	return &GroupedChannels{make(map[string]map[chan []byte]bool), new(sync.Mutex)}
}

type GroupedChannels struct {
	channels map[string]map[chan []byte]bool //{key => {channel => true, ...}, ...}
	mutex    *sync.Mutex
}

func keyify(keys []string) string {
	nonEmptyKeys := []string{}
	for i := range keys {
		if keys[i] != "" {
			nonEmptyKeys = append(nonEmptyKeys, keys[i])
		}
	}
	return strings.Join(nonEmptyKeys, ":")
}

func (gc *GroupedChannels) Add(channel chan []byte, keys ...string) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	if gc.channels[keyify(keys)] == nil {
		gc.channels[keyify(keys)] = make(map[chan []byte]bool)
	}
	gc.channels[keyify(keys)][channel] = true

}

func (gc *GroupedChannels) Get(keys ...string) []chan []byte {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	result := make([]chan []byte, 0, len(gc.channels[keyify(keys)]))

	for channel, _ := range gc.channels[keyify(keys)] {
		result = append(result, channel)
	}

	return result
}

func (gc *GroupedChannels) Delete(channel chan []byte) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	for _, channels := range gc.channels {
		delete(channels, channel)
	}
}

func (gc *GroupedChannels) NumberOfChannels() (numberOfChannels int) {
	for _, channels := range gc.channels {
		numberOfChannels += len(channels)
	}
	return numberOfChannels
}
