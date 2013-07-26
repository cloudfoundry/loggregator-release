package cfsink

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"instrumentor"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type cfSink struct {
	clientIdentifier string
	messageSentCount *uint64
}

func newCfSink(spaceId string, appId string, cfSinkServer *cfSinkServer, ws *websocket.Conn, clientAddress net.Addr) *cfSink {
	clientIdentifier := strings.Join([]string{spaceId, appId}, ":")

	sink := &cfSink{clientIdentifier, new(uint64)}

	sinkInstrumentor := instrumentor.NewInstrumentor(5*time.Second, gosteno.LOG_DEBUG, cfSinkServer.logger)
	stopChan := sinkInstrumentor.Instrument(sink)
	defer sinkInstrumentor.StopInstrumentation(stopChan)

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
		err := websocket.Message.Send(ws, data)
		if err != nil {
			cfSinkServer.logger.Infof("Tail client %s must have gone away %s", clientAddress, err)
			break
		}
		atomic.AddUint64(sink.messageSentCount, 1)
	}
	return sink
}

func (sinker *cfSink) DumpData() []instrumentor.PropVal {
	return []instrumentor.PropVal{
		instrumentor.PropVal{
			"SentMessageCount for " + sinker.clientIdentifier,
			strconv.FormatUint(atomic.LoadUint64(sinker.messageSentCount), 10)}}
}
