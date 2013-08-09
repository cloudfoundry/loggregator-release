package messagestore

import (
	"container/ring"
	"bytes"
	"sync"
	"encoding/binary"
)

type Message struct {
	length uint32
	payload []byte
}


func NewMessageStore(size int) *MessageStore {
	return &MessageStore{
		size: size,
		entries: make(map[string]map[string]*ring.Ring),
	}
}

type MessageStore struct{
	size int
	entries map[string]map[string]*ring.Ring
	sync.RWMutex
}

func (ms *MessageStore) Add(data []byte, spaceId string, appId string) {
	ms.Lock()
	defer ms.Unlock()
	apps, found := ms.entries[spaceId]

	if !found {
		apps = make(map[string]*ring.Ring)
		ms.entries[spaceId] = apps
	}

	messages, found := apps[appId]

	if !found {
		messages = ring.New(ms.size)
	}

	messages.Value = Message{uint32(len(data)), data}
	apps[appId] = messages.Next()
	return
}

func (ms *MessageStore) DumpFor(spaceId string, appId string) ([]byte) {
	ms.RLock()
	defer ms.RUnlock()
	buffer := bytes.NewBufferString("")

	writeMessages := func(m interface {}) {
		message, _ := m.(Message)
		binary.Write(buffer, binary.BigEndian, message.length)
		buffer.Write(message.payload)
	}

	if appId == "" {
		for _, messages := range ms.entries[spaceId] {
			messages.Do(writeMessages)
		}
	} else {
		messages := ms.entries[spaceId][appId]
		messages.Do(writeMessages)
	}

	return buffer.Bytes()
}
