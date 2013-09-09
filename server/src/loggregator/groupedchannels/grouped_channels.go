package groupedchannels

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"sync"
)

func NewGroupedChannels() *GroupedChannels {
	return &GroupedChannels{make(map[string]*node), new(sync.RWMutex)}
}

func newNode() *node {
	return &node{channelSet: make(map[chan *logmessage.Message]bool)}
}

type node struct {
	channelSet map[chan *logmessage.Message]bool
}

func (n *node) addChannel(c chan *logmessage.Message) {
	n.channelSet[c] = true
}

type GroupedChannels struct {
	apps map[string]*node
	*sync.RWMutex
}

func (gc *GroupedChannels) Register(c chan *logmessage.Message, appId string) {
	gc.Lock()
	defer gc.Unlock()

	if appId != "" {
		app, found := gc.apps[appId]
		if !found {
			app = newNode()
			gc.apps[appId] = app
		}
		app.addChannel(c)
	}
}

func (gc *GroupedChannels) For(appId string) (results []chan *logmessage.Message) {
	gc.RLock()
	defer gc.RUnlock()

	results = make([]chan *logmessage.Message, 0)

	app, found := gc.apps[appId]

	if found {
		for c, _ := range app.channelSet {
			results = append(results, c)
		}
	}

	return results
}

func (gc *GroupedChannels) Delete(c chan *logmessage.Message) {
	gc.Lock()
	defer gc.Unlock()
	for _, app := range gc.apps {
		delete(app.channelSet, c)
	}
}

func (gc *GroupedChannels) NumberOfChannels() (numberOfChannels int) {
	for _, app := range gc.apps {
		numberOfChannels += len(app.channelSet)
	}
	return numberOfChannels
}
