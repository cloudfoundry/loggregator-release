package messagestore

import (
	"container/ring"
	"bytes"
	"sync"
	"encoding/binary"
	"loggregator/logtarget"
)

type Message struct {
	length  uint32
	payload []byte
}

func newNode(size int) *node {
	return &node{childNodes: make(map[string]*node), entries: ring.New(size)}
}

type node struct {
	childNodes map[string]*node
	entries *ring.Ring
}

func (n *node) addData(d []byte) {
	n.entries.Value = Message{uint32(len(d)), d}
	n.entries = n.entries.Next()
}

func NewMessageStore(size int) *MessageStore {
	return &MessageStore{
		size: size,
		orgs: make(map[string]*node),
	}
}

type MessageStore struct{
	size int
	orgs map[string]*node
	sync.RWMutex
}

func (ms *MessageStore) Add(data []byte, lt *logtarget.LogTarget) {
	ms.Lock()
	defer ms.Unlock()

	if lt.OrgId != "" {
		org, found := ms.orgs[lt.OrgId]
		if !found {
			org = newNode(ms.size)
			ms.orgs[lt.OrgId] = org
		}
		if lt.SpaceId != "" {
			space, found := org.childNodes[lt.SpaceId]
			if !found {
				space = newNode(ms.size)
				org.childNodes[lt.SpaceId] = space
			}
			if lt.AppId != "" {
				app, found := space.childNodes[lt.AppId]
				if !found {
					app = newNode(ms.size)
					space.childNodes[lt.AppId] = app
				}
				app.addData(data)
			} else {
				space.addData(data)
			}
		} else {
			org.addData(data)
		}
	}

	return
}

func (ms *MessageStore) DumpFor(lt *logtarget.LogTarget) ([]byte) {
	ms.RLock()
	defer ms.RUnlock()

	buffer := bytes.NewBufferString("")

	writeEntries := func(m interface {}) {
		message, _ := m.(Message)
		if message.length > 0 {
			binary.Write(buffer, binary.BigEndian, message.length)
			buffer.Write(message.payload)
		}
	}

	org, orgFound := ms.orgs[lt.OrgId]
	if !orgFound {
		return buffer.Bytes()
	}

	space, spaceFound := org.childNodes[lt.SpaceId]
	if spaceFound {
		app, appFound := space.childNodes[lt.AppId]
		if appFound {
			app.entries.Do(writeEntries)
		}else {
			space.entries.Do(writeEntries)
			for _, app := range space.childNodes {
				app.entries.Do(writeEntries)
			}
		}
	} else {
		org.entries.Do(writeEntries)
		for _, space := range org.childNodes {
			space.entries.Do(writeEntries)
			for _, app := range space.childNodes {
				app.entries.Do(writeEntries)
			}
		}
	}
	return buffer.Bytes()
}
