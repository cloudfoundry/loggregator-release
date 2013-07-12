package loggregator

import "sync"

func newGroupedChannels() *groupedChannels {
	return &groupedChannels{make(map[string]map[chan []byte]bool), new(sync.Mutex)}
}

type groupedChannels struct {
	channels map[string]map[chan []byte]bool //{key => {channel => true, ...}, ...}
	mutex    *sync.Mutex
}

func (gc *groupedChannels) add(channel chan []byte, keys ...string) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	if gc.channels[keys[0]] == nil {
		gc.channels[keys[0]] = make(map[chan []byte]bool)
	}
	gc.channels[keys[0]][channel] = true

}

func (gc *groupedChannels) get(keys ...string) []chan []byte {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	result := make([]chan []byte, 0, len(gc.channels[keys[0]]))

	for channel, _ := range gc.channels[keys[0]] {
		result = append(result, channel)
	}

	return result
}

func (gc *groupedChannels) delete(channel chan []byte, keys ...string) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	delete(gc.channels[keys[0]], channel)
}
