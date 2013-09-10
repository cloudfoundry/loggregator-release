package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/authorization"
	"loggregator/groupedchannels"
	"loggregator/messagestore"
	"loggregator/sinks"
	"net/http"
	"time"
)

const (
	TAIL_PATH = "/tail/"
	DUMP_PATH = "/dump/"
)

type sinkServer struct {
	parsedMessageChan chan *logmessage.Message
	sinkCloseChan     chan sinks.Sink

	activeSinksChans *groupedchannels.GroupedChannels
	messageStore     *messagestore.MessageStore
	drainUrlsForApps map[string]map[string]sinks.Sink

	authorize         authorization.LogAccessAuthorizer
	keepAliveInterval time.Duration

	logger *gosteno.Logger
}

func NewSinkServer(messageStore *messagestore.MessageStore, logger *gosteno.Logger, authorize authorization.LogAccessAuthorizer, keepAliveInterval time.Duration) *sinkServer {
	activeSinks := groupedchannels.NewGroupedChannels()
	sinkCloseChan := make(chan sinks.Sink, 4)
	drainUrlsForApps := make(map[string]map[string]sinks.Sink, 100)
	messageChannel := make(chan *logmessage.Message, 2048)

	return &sinkServer{
		logger:            logger,
		parsedMessageChan: messageChannel,
		drainUrlsForApps:  drainUrlsForApps,
		activeSinksChans:  activeSinks,
		authorize:         authorize,
		sinkCloseChan:     sinkCloseChan,
		keepAliveInterval: keepAliveInterval,
		messageStore:      messageStore}
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

	sinkServer.activeSinksChans.Register(s.Channel(), appId)
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

func (sinkServer *sinkServer) registerDrainUrls(appId string, drainUrls []string) {
	if len(drainUrls) == 0 {
		for _, sink := range sinkServer.drainUrlsForApps[appId] {
			sinkServer.activeSinksChans.Delete(sink.Channel())
			close(sink.Channel())
		}
		delete(sinkServer.drainUrlsForApps, appId)
		return
	}
	if sinkServer.drainUrlsForApps[appId] == nil {
		sinkServer.drainUrlsForApps[appId] = make(map[string]sinks.Sink, len(drainUrls))
	}
	for _, sink := range sinkServer.drainUrlsForApps[appId] {
		if contains(sink.Identifier(), drainUrls) {
			continue
		}
		sinkServer.activeSinksChans.Delete(sink.Channel())
		close(sink.Channel())
		delete(sinkServer.drainUrlsForApps[appId], sink.Identifier())
	}
	for _, drainUrl := range drainUrls {
		if sinkServer.drainUrlsForApps[appId][drainUrl] == nil {
			s, err := sinks.NewSyslogSink(appId, drainUrl, sinkServer.logger)
			if err != nil {
				sinkServer.logger.Error(err.Error())
				continue
			}
			go s.Run(sinkServer.sinkCloseChan)
			sinkServer.activeSinksChans.Register(s.Channel(), appId)
			sinkServer.drainUrlsForApps[appId][drainUrl] = s
		}
	}
}

func (sinkServer *sinkServer) relayMessagesToAllSinks() {
	for {
		select {
		case s := <-sinkServer.sinkCloseChan:
			sinkServer.activeSinksChans.Delete(s.Channel())
			delete(sinkServer.drainUrlsForApps[s.AppId()], s.Identifier())
			close(s.Channel())
			sinkServer.logger.Infof("SinkServer: Sink with channel %v requested closing. Closed it.", s.Channel())
		case receivedMessage := <-sinkServer.parsedMessageChan:
			sinkServer.logger.Debugf("SinkServer: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

			//drain management
			appId := receivedMessage.GetLogMessage().GetAppId()
			if receivedMessage.GetLogMessage().GetSourceType() == logmessage.LogMessage_WARDEN_CONTAINER {
				sinkServer.registerDrainUrls(appId, receivedMessage.GetLogMessage().GetDrainUrls())
			}

			//dump management
			sinkServer.messageStore.Add(receivedMessage, appId)

			//send to drains and sinks
			sinkServer.logger.Debugf("SinkServer: Searching for sinks with appId [%s].", appId)
			for _, sinkChan := range sinkServer.activeSinksChans.For(appId) {
				sinkServer.logger.Debugf("SinkServer: Sending Message to channel %v for sinks targeting [%s].", sinkChan, appId)
				sinkChan <- receivedMessage
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
			sinkServer.logger.Error(fmt.Sprintf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data))
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
	return []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfSinks", Value: sinkServer.activeSinksChans.NumberOfChannels()},
	}
}

func (sinkServer *sinkServer) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "sinkServer",
		Metrics: sinkServer.metrics(),
	}
}
