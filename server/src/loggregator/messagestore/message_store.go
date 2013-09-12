package messagestore

import (
	"bytes"
	"container/ring"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"sync"
)

func newNode(size int) *node {
	return &node{entries: ring.New(size)}
}

type node struct {
	entries *ring.Ring
}

func (n *node) addData(d *logmessage.Message) {
	n.entries.Value = *d
	n.entries = n.entries.Next()
}

func NewMessageStore(size int) *MessageStore {
	return &MessageStore{
		size: size,
		apps: make(map[string]*node),
	}
}

type MessageStore struct {
	size int
	apps map[string]*node
	sync.RWMutex
}

func (ms *MessageStore) Add(message *logmessage.Message, appId string) {
	ms.Lock()
	defer ms.Unlock()

	if appId != "" {
		app, found := ms.apps[appId]
		if !found {
			app = newNode(ms.size)
			ms.apps[appId] = app
		}
		app.addData(message)
	}
}

func (ms *MessageStore) DumpFor(appId string) []byte {
	ms.RLock()
	defer ms.RUnlock()

	buffer := bytes.NewBufferString("")

	writeEntries := func(m interface{}) {
		message, _ := m.(logmessage.Message)

		if message.GetRawMessageLength() > 0 {
			logmessage.DumpMessage(message, buffer)
		}
	}
	app, appFound := ms.apps[appId]
	if !appFound {
		return buffer.Bytes()
	}
	app.entries.Do(writeEntries)

	return buffer.Bytes()
}
