package groupedchannels

import (
	"loggregator/logtarget"
	"sync"
)

func NewGroupedChannels() *GroupedChannels {
	return &GroupedChannels{make(map[string]*node), new(sync.RWMutex)}
}

func newNode() *node {
	return &node{childNodes: make(map[string]*node), channelSet: make(map[chan []byte]bool)}
}

type node struct {
	childNodes map[string]*node
	channelSet map[chan []byte]bool
}

func (n *node) addChannel(c chan []byte) {
	n.channelSet[c] = true
}

type GroupedChannels struct {
	orgs map[string]*node
	*sync.RWMutex
}

func (gc *GroupedChannels) Register(c chan []byte, lt *logtarget.LogTarget) {
	gc.Lock()
	defer gc.Unlock()

	if lt.OrgId != "" {
		org, found := gc.orgs[lt.OrgId]
		if !found {
			org = newNode()
			gc.orgs[lt.OrgId] = org
		}
		if lt.SpaceId != "" {
			space, found := org.childNodes[lt.SpaceId]
			if !found {
				space = newNode()
				org.childNodes[lt.SpaceId] = space
			}
			if lt.AppId != "" {
				app, found := space.childNodes[lt.AppId]
				if !found {
					app = newNode()
					space.childNodes[lt.AppId] = app
				}
				app.addChannel(c)
			} else {
				space.addChannel(c)
			}
		} else {
			org.addChannel(c)
		}
	}
}

func (gc *GroupedChannels) For(lt *logtarget.LogTarget) (results []chan []byte) {
	gc.RLock()
	defer gc.RUnlock()

	results = make([]chan []byte, 0)

	org, found := gc.orgs[lt.OrgId]

	if found {
		for c, _ := range org.channelSet {
			results = append(results, c)
		}
	} else {
		return
	}

	space, found := org.childNodes[lt.SpaceId]

	if found {
		for c, _ := range space.channelSet {
			results = append(results, c)
		}
	} else {
		return
	}

	app, found := space.childNodes[lt.AppId]

	if found {
		for c, _ := range app.channelSet {
			results = append(results, c)
		}
	}
	return
}

func (gc *GroupedChannels) Delete(c chan []byte) {
	gc.Lock()
	defer gc.Unlock()

	for _, org := range gc.orgs {
		delete(org.channelSet, c)
		for _, space := range org.childNodes {
			delete(space.channelSet, c)
			for _, app := range space.childNodes {
				delete(app.channelSet, c)
			}
		}
	}
}

func (gc *GroupedChannels) NumberOfChannels() (numberOfChannels int) {
	for _, org := range gc.orgs {
		numberOfChannels += len(org.channelSet)
		for _, space := range org.childNodes {
			numberOfChannels += len(space.channelSet)
			for _, app := range space.childNodes {
				numberOfChannels += len(app.channelSet)
			}
		}
	}
	return
}
