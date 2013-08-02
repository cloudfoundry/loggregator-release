package sink

import (
	"cfcomponent/instrumentation"
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"logMessage"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type sinkServer struct {
	logger            *gosteno.Logger
	dataChannel       chan []byte
	listenHost        string
	listenPath        string
	apiHost           string
	listenerChannels  *groupedChannels
	authorize         LogAccessAuthorizer
	sinkCloseChan     chan chan []byte
	keepAliveInterval time.Duration
}

func NewSinkServer(givenChannel chan []byte, logger *gosteno.Logger, listenHost string, listenPath string, apiHost string, authorize LogAccessAuthorizer, keepAliveInterval time.Duration) *sinkServer {
	listeners := newGroupedChannels()
	sinkCloseChan := make(chan chan []byte, 4)
	return &sinkServer{logger, givenChannel, listenHost, listenPath, apiHost, listeners, authorize, sinkCloseChan, keepAliveInterval}
}

func (sinkServer *sinkServer) sinkRelayHandler(ws *websocket.Conn) {
	extractAppIdAndAuthTokenFromUrl := func(u *url.URL) (string, string, string) {
		authorization := ""
		queryValues := u.Query()
		if len(queryValues["authorization"]) == 1 {
			authorization = queryValues["authorization"][0]
		}
		appId := ""
		spaceId := ""
		re := regexp.MustCompile("^" + sinkServer.listenPath + "spaces/([^/]+)(?:/apps/([^/]+))?$")
		result := re.FindStringSubmatch(u.Path)

		switch len(result) {
		case 2:
			spaceId = result[1]
		case 3:
			spaceId = result[1]
			appId = result[2]
		}

		return spaceId, appId, authorization
	}

	clientAddress := ws.RemoteAddr()

	spaceId, appId, authToken := extractAppIdAndAuthTokenFromUrl(ws.Request().URL)

	if spaceId == "" {
		message := fmt.Sprintf("Did not accept sink connection from %s without spaceId.", clientAddress)
		sinkServer.logger.Warn(message)
		return
	}
	if authToken == "" {
		message := fmt.Sprintf("Did not accept sink connection from %s without authorization.", clientAddress)
		sinkServer.logger.Warnf(message)
		return
	}

	if !sinkServer.authorize(sinkServer.apiHost, authToken, spaceId, appId, sinkServer.logger) {
		message := fmt.Sprintf("User not authorized to access space [%s].", spaceId)
		sinkServer.logger.Warn(message)
		return
	}

	sink := newCfSink(spaceId, appId, sinkServer.logger, ws, clientAddress, sinkServer.keepAliveInterval)

	listenerChannel := make(chan []byte)
	sinkServer.listenerChannels.add(listenerChannel, spaceId, appId)
	sink.Run(listenerChannel, sinkServer.sinkCloseChan)
}

func (sinkServer *sinkServer) relayMessagesToAllSinks() {
	extractReceivedSpaceAndAppId := func(data []byte) (string, string) {
		receivedMessage := &logMessage.LogMessage{}
		err := proto.Unmarshal(data, receivedMessage)
		if err != nil {
			sinkServer.logger.Debugf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data)
			return "", ""
		}
		return *receivedMessage.SpaceId, *receivedMessage.AppId
	}

	for {
		select {
		case sin := <-sinkServer.sinkCloseChan:
			sinkServer.listenerChannels.delete(sin)
			close(sin)
			sinkServer.logger.Info("A Tail client went have away. Closed it.")
		case data := <-sinkServer.dataChannel:
			sinkServer.logger.Debugf("Received %d bytes of data from agent listener.", len(data))
			receivedSpaceId, receivedAppId := extractReceivedSpaceAndAppId(data)
			sinkServer.logger.Debugf("Searching for channels with spaceId [%s] and appId [%s].", receivedSpaceId, receivedAppId)
			for _, listenerChannel := range sinkServer.listenerChannels.get(receivedSpaceId, receivedAppId) {
				sinkServer.logger.Debugf("Sending Message to channel %s for space [%s] and app [%s].", listenerChannel, receivedSpaceId, receivedAppId)
				listenerChannel <- data
			}
			sinkServer.logger.Debugf("Searching for channels with spaceId [%s].", receivedSpaceId)
			for _, listenerChannel := range sinkServer.listenerChannels.get(receivedSpaceId) {
				sinkServer.logger.Debugf("Sending Message to channel %s for space [%s].", listenerChannel, receivedSpaceId)
				listenerChannel <- data
			}
			sinkServer.logger.Debugf("Done sending message to tail clients.")
		}
	}
}

func (sinkServer *sinkServer) Start() {
	go sinkServer.relayMessagesToAllSinks()

	sinkServer.logger.Infof("Listening on port %s", sinkServer.listenHost)
	http.Handle(sinkServer.listenPath, websocket.Handler(sinkServer.sinkRelayHandler))
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
