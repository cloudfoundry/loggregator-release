package loggregator

func newGroupedChannels() *groupedChannels {
	return &groupedChannels{make(map[string]map[chan []byte]bool)}
}

type groupedChannels struct {
	channels map[string]map[chan []byte]bool //{key => {channel => true, ...}, ...}
}

func (gc *groupedChannels) add(key string, channel chan []byte) {
	if gc.channels[key] == nil {
		gc.channels[key] = make(map[chan []byte]bool)
	}
	gc.channels[key][channel] = true
}

func (gc *groupedChannels) get(key string) []chan []byte {
	result := make([]chan []byte, 0, len(gc.channels[key]))

	for channel, _ := range gc.channels[key] {
		result = append(result, channel)
	}

	return result
}

func (gc *groupedChannels) delete(key string, channel chan []byte) {
	delete(gc.channels[key], channel)
}
