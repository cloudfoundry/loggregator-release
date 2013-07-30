package sink

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

type sink struct {
	logger           *gosteno.Logger
	spaceId          string
	appId            string
	ws               *websocket.Conn
	clientAddress    net.Addr
	sentMessageCount *uint64
	sentByteCount    *uint64
}

func newCfSink(spaceId string, appId string, givenLogger *gosteno.Logger, ws *websocket.Conn, clientAddress net.Addr) *sink {
	return &sink{givenLogger, spaceId, appId, ws, clientAddress, new(uint64), new(uint64)}
}

func (sink *sink) clientIdentifier() string {
	return strings.Join([]string{sink.spaceId, sink.appId}, ":")
}

func (sink *sink) Start() (listenerChannel chan []byte, closeChannel chan bool) {
	sinkInstrumentor := instrumentor.NewInstrumentor(5*time.Second, gosteno.LOG_DEBUG, sink.logger)
	stopChan := sinkInstrumentor.Instrument(sink)
	defer sinkInstrumentor.StopInstrumentation(stopChan)

	if sink.appId != "" {
		sink.logger.Debugf("Adding Tail client %s for space [%s] and app [%s].", sink.clientAddress, sink.spaceId, sink.appId)
	} else {
		sink.logger.Debugf("Adding Tail client %s for space [%s].", sink.clientAddress, sink.spaceId)
	}

	listenerChannel = make(chan []byte)
	closeChannel = make(chan bool)

	go func() {
		for {
			sink.logger.Debugf("Tail client %s is waiting for data", sink.clientAddress)
			sink.logger.Debugf("My channel is %v", listenerChannel)
			data := <-listenerChannel
			sink.logger.Debugf("Tail client %s got %d bytes", sink.clientAddress, len(data))
			sink.logger.Debugf("Server client conn before send: %v - %v", sink.ws.IsClientConn(), sink.ws.IsServerConn())
			err := websocket.Message.Send(sink.ws, data)
			sink.logger.Debugf("Server client conn after send: %v - %v", sink.ws.IsClientConn(), sink.ws.IsServerConn())
			if err != nil {
				sink.logger.Debugf("Error when sending data to sink %s. Err: ", sink.clientAddress, err)
				closeChannel <- true
				return
			}
			atomic.AddUint64(sink.sentMessageCount, 1)
			atomic.AddUint64(sink.sentByteCount, uint64(len(data)))
		}
	}()
	return listenerChannel, closeChannel
}

func (sink *sink) DumpData() []instrumentor.PropVal {
	return []instrumentor.PropVal{
		instrumentor.PropVal{
			"SentMessageCount for " + sink.clientIdentifier(),
			strconv.FormatUint(atomic.LoadUint64(sink.sentMessageCount), 10)},
		instrumentor.PropVal{
			"SentByteCount for " + sink.clientIdentifier(),
			strconv.FormatUint(atomic.LoadUint64(sink.sentByteCount), 10)},
	}
}
