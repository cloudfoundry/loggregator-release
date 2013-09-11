package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/authorization"
	"loggregator/groupedsinks"
	"loggregator/messagestore"
	"loggregator/sinks"
	"net/http"
	"sync"
	"time"
)

const (
	TAIL_PATH = "/tail/"
	DUMP_PATH = "/dump/"
)

type sinkServer struct {
	parsedMessageChan  chan *logmessage.Message
	sinkCloseChan      chan sinks.Sink
	sinkOpenChan       chan sinks.Sink
	activeSinksCounter int
	messageStore       *messagestore.MessageStore

	authorize         authorization.LogAccessAuthorizer
	keepAliveInterval time.Duration

	logger *gosteno.Logger

	*sync.RWMutex
}

func NewSinkServer(messageStore *messagestore.MessageStore, logger *gosteno.Logger, authorize authorization.LogAccessAuthorizer, keepAliveInterval time.Duration) *sinkServer {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	messageChannel := make(chan *logmessage.Message, 2048)

	return &sinkServer{
		logger:            logger,
		parsedMessageChan: messageChannel,
		authorize:         authorize,
		sinkCloseChan:     sinkCloseChan,
		sinkOpenChan:      sinkOpenChan,
		keepAliveInterval: keepAliveInterval,
		messageStore:      messageStore,
		RWMutex:           &sync.RWMutex{}}
}

func (sinkServer *sinkServer) sinkRelayHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()

	appId := appid.FromUrl(ws.Request().URL)
	authToken := ws.Request().Header.Get("Authorization")

	if appId == "" {
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

	if !sinkServer.authorize(authToken, appId, sinkServer.logger) {
		message := fmt.Sprintf("SinkServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		sinkServer.logger.Warn(message)
		ws.CloseWithStatus(4002)
		return
	}

	s := sinks.NewWebsocketSink(appId, sinkServer.logger, ws, clientAddress, sinkServer.keepAliveInterval)
	sinkServer.logger.Debugf("SinkServer: Requesting a wss sink for app %s", s.AppId())
	sinkServer.sinkOpenChan <- s
	s.Run(sinkServer.sinkCloseChan)
}

func (sinkServer *sinkServer) dumpHandler(rw http.ResponseWriter, req *http.Request) {
	appId := appid.FromUrl(req.URL)
	authToken := req.Header.Get("Authorization")

	if appId == "" {
		sinkServer.logger.Warn("SinkServer: Did not accept dump connection with invalid app id.")
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if !sinkServer.authorize(authToken, appId, sinkServer.logger) {
		message := fmt.Sprintf("SinkServer: Auth token [%s] not authorized to access target [%s].", authToken, appId)
		sinkServer.logger.Warn(message)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.Write(sinkServer.messageStore.DumpFor(appId))
}

func contains(valueToFind string, values []string) bool {
	for _, value := range values {
		if valueToFind == value {
			return true
		}
	}
	return false
}

func (sinkServer *sinkServer) manageDrainUrls(activeSinks *groupedsinks.GroupedSinks, appId string, drainUrls []string) {
	//delete all drains for app
	if len(drainUrls) == 0 {
		for _, sink := range activeSinks.DrainsFor(appId) {
			sinkServer.unregisterSink(sink, activeSinks)
		}
		return
	}

	//delete all drains that were not sent
	for _, sink := range activeSinks.DrainsFor(appId) {
		if contains(sink.Identifier(), drainUrls) {
			continue
		}
		sinkServer.unregisterSink(sink, activeSinks)
	}

	//add all drains that didn't exist
	for _, drainUrl := range drainUrls {
		if activeSinks.DrainFor(appId, drainUrl) == nil {
			s := sinks.NewSyslogSink(appId, drainUrl, sinkServer.logger)
			go s.Run(sinkServer.sinkCloseChan)
			sinkServer.registerSink(s, activeSinks)
		}
	}
}

func (sinkServer *sinkServer) registerSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) {
	sinkServer.Lock()
	defer sinkServer.Unlock()

	activeSinks.Register(s, s.AppId())
	sinkServer.activeSinksCounter++
	sinkServer.logger.Infof("SinkServer: Sink with channel %v requested. Opened it.", s.Channel())
}

func (sinkServer *sinkServer) unregisterSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) {
	sinkServer.Lock()
	defer sinkServer.Unlock()

	activeSinks.Delete(s)
	close(s.Channel())
	sinkServer.activeSinksCounter--
	sinkServer.logger.Infof("SinkServer: Sink with channel %v requested closing. Closed it.", s.Channel())
}

func (sinkServer *sinkServer) relayMessagesToAllSinks() {
	activeSinks := groupedsinks.NewGroupedSinks()

	for {
		select {
		case s := <-sinkServer.sinkOpenChan:
			sinkServer.registerSink(s, activeSinks)
		case s := <-sinkServer.sinkCloseChan:
			sinkServer.unregisterSink(s, activeSinks)
		case receivedMessage := <-sinkServer.parsedMessageChan:
			sinkServer.logger.Debugf("SinkServer: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

			//drain management
			appId := receivedMessage.GetLogMessage().GetAppId()
			if receivedMessage.GetLogMessage().GetSourceType() == logmessage.LogMessage_WARDEN_CONTAINER {
				sinkServer.manageDrainUrls(activeSinks, appId, receivedMessage.GetLogMessage().GetDrainUrls())
			}

			//dump management
			sinkServer.messageStore.Add(receivedMessage, appId)

			//send to drains and sinks
			sinkServer.logger.Debugf("SinkServer: Searching for sinks with appId [%s].", appId)
			for _, s := range activeSinks.For(appId) {
				sinkServer.logger.Debugf("SinkServer: Sending Message to channel %v for sinks targeting [%s].", s, appId)
				s.Channel() <- receivedMessage
			}
			sinkServer.logger.Debugf("SinkServer: Done sending message to tail clients.")
		}
	}
}

func (sinkServer *sinkServer) parseMessages(incomingProtobufChan <-chan []byte) {
	for {
		data := <-incomingProtobufChan
		message, err := logmessage.ParseMessage(data)
		if err != nil {
			sinkServer.logger.Errorf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data)
			continue
		}
		sinkServer.parsedMessageChan <- message
	}
}

func (sinkServer *sinkServer) Start(incomingProtobufChan <-chan []byte, apiEndpoint string) {
	go sinkServer.parseMessages(incomingProtobufChan)
	go sinkServer.relayMessagesToAllSinks()

	sinkServer.logger.Infof("SinkServer: Listening for sinks at %s", apiEndpoint)
	http.Handle(TAIL_PATH, websocket.Handler(sinkServer.sinkRelayHandler))
	http.HandleFunc(DUMP_PATH, sinkServer.dumpHandler)
	err := http.ListenAndServe(apiEndpoint, nil)
	if err != nil {
		panic(err)
	}
}

func (sinkServer *sinkServer) metrics() []instrumentation.Metric {
	sinkServer.RLock()
	defer sinkServer.RUnlock()

	return []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfSinks", Value: sinkServer.activeSinksCounter},
	}
}

func (sinkServer *sinkServer) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "sinkServer",
		Metrics: sinkServer.metrics(),
	}
}
