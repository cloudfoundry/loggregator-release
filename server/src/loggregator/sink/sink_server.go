package sink

import (
	"cfcomponent/instrumentation"
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"loggregator/authorization"
	"loggregator/groupedchannels"
	"loggregator/logtarget"
	"loggregator/messagestore"
	"net/http"
	"time"
)

const (
	TAIL_PATH = "/tail/"
	DUMP_PATH = "/dump/"
)

type sinkServer struct {
	logger            *gosteno.Logger
	dataChannel       chan []byte
	listenHost        string
	listenerChannels  *groupedchannels.GroupedChannels
	authorize         authorization.LogAccessAuthorizer
	sinkCloseChan     chan chan []byte
	keepAliveInterval time.Duration
	messageStore      *messagestore.MessageStore
}

func NewSinkServer(givenChannel chan []byte, messageStore *messagestore.MessageStore, logger *gosteno.Logger, listenHost string, authorize authorization.LogAccessAuthorizer, keepAliveInterval time.Duration) *sinkServer {
	listeners := groupedchannels.NewGroupedChannels()
	sinkCloseChan := make(chan chan []byte, 4)
	return &sinkServer{logger, givenChannel, listenHost, listeners, authorize, sinkCloseChan, keepAliveInterval, messageStore}
}

func (sinkServer *sinkServer) sinkRelayHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()

	target := logtarget.FromUrl(ws.Request().URL)
	authToken := ws.Request().Header.Get("Authorization")

	if !target.IsValid() {
		message := fmt.Sprintf("SinkServer: Did not accept sink connection with invalid app id: %s.", clientAddress)
		sinkServer.logger.Warn(message)
		ws.CloseWithStatus(4000)
		return
	}

	if authToken == "" {
		message := fmt.Sprintf("SinkServer: Did not accept sink connection from %s without authorization.", clientAddress)
		sinkServer.logger.Warnf(message)
		ws.CloseWithStatus(4001)
		return
	}

	if !sinkServer.authorize(authToken, target, sinkServer.logger) {
		message := fmt.Sprintf("SinkServer: Auth token [%s] not authorized to access target [%s].", authToken, target)
		sinkServer.logger.Warn(message)
		ws.CloseWithStatus(4002)
		return
	}

	sink := newCfSink(target, sinkServer.logger, ws, clientAddress, sinkServer.keepAliveInterval)

	sinkServer.listenerChannels.Register(sink.listenerChannel, target)
	sink.Run(sinkServer.sinkCloseChan)
}

func (sinkServer *sinkServer) dumpHandler(rw http.ResponseWriter, req *http.Request) {
	target := logtarget.FromUrl(req.URL)
	authToken := req.Header.Get("Authorization")

	if !target.IsValid() {
		sinkServer.logger.Warn("SinkServer: Did not accept dump connection with invalid app id.")
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if !sinkServer.authorize(authToken, target, sinkServer.logger) {
		message := fmt.Sprintf("SinkServer: Auth token [%s] not authorized to access target [%s].", authToken, target.AppId)
		sinkServer.logger.Warn(message)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.Write(sinkServer.messageStore.DumpFor(target))
}

func (sinkServer *sinkServer) relayMessagesToAllSinks() {
	for {
		select {
		case sin := <-sinkServer.sinkCloseChan:
			sinkServer.listenerChannels.Delete(sin)
			close(sin)
			sinkServer.logger.Infof("SinkServer: Sink with channel %v requested closing. Closed it.", sin)
		case data := <-sinkServer.dataChannel:
			sinkServer.logger.Debugf("SinkServer: Received %d bytes of data from agent listener.", len(data))
			target, err := logtarget.FromLogMessage(data)
			if err != nil {
				sinkServer.logger.Error(err.Error())
			} else {
				sinkServer.messageStore.Add(data, target)
				sinkServer.logger.Debugf("SinkServer: Searching for sinks with target [%s].", target)
				for _, listenerChannel := range sinkServer.listenerChannels.For(target) {
					sinkServer.logger.Debugf("SinkServer: Sending Message to channel %v for sinks targeting [%s].", listenerChannel, target)
					listenerChannel <- data
				}
				sinkServer.logger.Debugf("SinkServer: Done sending message to tail clients.")
			}
		}
	}
}

func (sinkServer *sinkServer) Start() {
	go sinkServer.relayMessagesToAllSinks()

	sinkServer.logger.Infof("SinkServer: Listening on port %s", sinkServer.listenHost)
	http.Handle(TAIL_PATH, websocket.Handler(sinkServer.sinkRelayHandler))
	http.HandleFunc(DUMP_PATH, sinkServer.dumpHandler)
	err := http.ListenAndServe(sinkServer.listenHost, nil)
	if err != nil {
		panic(err)
	}
}

func (sinkServer *sinkServer) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name: "sinkServer",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{"numberOfSinks", sinkServer.listenerChannels.NumberOfChannels()},
		},
	}
}
