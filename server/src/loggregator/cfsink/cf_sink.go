package cfsink

import (
	"net/http"
	"strings"
	"sync/atomic"
)

type cfSink struct {
	clientIdentifier string
	messageSentCount *uint64
}

func newCfSink(spaceId string, appId string, cfSinkServer *cfSinkServer, rw *http.ResponseWriter, f *http.Flusher, clientAddress string) *cfSink {
	clientIdentifier := strings.Join([]string{spaceId, appId}, ":")

	sinker := &cfSink{clientIdentifier, new(uint64)}

	listenerChannel := make(chan []byte)
	if appId != "" {
		cfSinkServer.logger.Debugf("Adding Tail client %s for space [%s] and app [%s].", clientAddress, spaceId, appId)
		cfSinkServer.listenerChannels.add(listenerChannel, spaceId, appId)
		defer cfSinkServer.listenerChannels.delete(listenerChannel, spaceId, appId)
	} else {
		cfSinkServer.logger.Debugf("Adding Tail client %s for space [%s].", clientAddress, spaceId)
		cfSinkServer.listenerChannels.add(listenerChannel, spaceId)
		defer cfSinkServer.listenerChannels.delete(listenerChannel, spaceId)
	}

	for {
		cfSinkServer.logger.Infof("Tail client %s is waiting for data", clientAddress)
		data := <-listenerChannel
		cfSinkServer.logger.Debugf("Tail client %s got %d bytes", clientAddress, len(data))
		_, err := (*rw).Write(data)
		if err != nil {
			cfSinkServer.logger.Infof("Tail client %s must have gone away %s", clientAddress, err)
			break
		}
		(*f).Flush()
		atomic.AddUint64(sinker.messageSentCount, 1)
	}
	return sinker
}
