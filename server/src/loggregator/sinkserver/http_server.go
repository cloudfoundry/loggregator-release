package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/authorization"
	"loggregator/sinks"
	"net"
	"net/http"
	"time"
)

const (
	TAIL_PATH = "/tail/"
	DUMP_PATH = "/dump/"
)

type httpServer struct {
	messageRouter     *messageRouter
	authorize         authorization.LogAccessAuthorizer
	keepAliveInterval time.Duration
	logger            *gosteno.Logger
}

func NewHttpServer(messageRouter *messageRouter, authorize authorization.LogAccessAuthorizer, keepAliveInterval time.Duration, logger *gosteno.Logger) *httpServer {
	return &httpServer{messageRouter, authorize, keepAliveInterval, logger}
}

func (httpServer *httpServer) Start(incomingProtobufChan <-chan []byte, apiEndpoint string) {
	go httpServer.parseMessages(incomingProtobufChan)

	httpServer.logger.Infof("HttpServer: Listening for sinks at %s", apiEndpoint)
	http.Handle(TAIL_PATH, websocket.Handler(httpServer.websocketSinkHandler))
	http.Handle(DUMP_PATH, websocket.Handler(httpServer.dumpSinkHandler))
	err := http.ListenAndServe(apiEndpoint, nil)
	if err != nil {
		panic(err)
	}
}

func (httpServer *httpServer) parseMessages(incomingProtobufChan <-chan []byte) {
	for {
		data := <-incomingProtobufChan
		message, err := logmessage.ParseMessage(data)
		if err != nil {
			httpServer.logger.Errorf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data)
			continue
		}
		httpServer.messageRouter.parsedMessageChan <- message
	}
}

func (httpServer *httpServer) isAuthorized(appId, authToken string, clientAddress net.Addr) int {
	if appId == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection with invalid app id: %s.", clientAddress)
		httpServer.logger.Warn(message)
		return 4000
	}

	if authToken == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection from %s without authorization.", clientAddress)
		httpServer.logger.Warnf(message)
		return 4001
	}

	if !httpServer.authorize(authToken, appId, httpServer.logger) {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		httpServer.logger.Warn(message)
		return 4002
	}

	return 0
}

func (httpServer *httpServer) websocketSinkHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	appId := appid.FromUrl(ws.Request().URL)
	authToken := ws.Request().Header.Get("Authorization")

	if code := httpServer.isAuthorized(appId, authToken, clientAddress); code > 200 {
		ws.CloseWithStatus(code)
		return
	}

	s := sinks.NewWebsocketSink(appId, httpServer.logger, ws, clientAddress, httpServer.keepAliveInterval)
	httpServer.logger.Debugf("HttpServer: Requesting a wss sink for app %s", s.AppId())
	httpServer.messageRouter.sinkOpenChan <- s

	s.Run(httpServer.messageRouter.sinkCloseChan)
}

func (httpServer *httpServer) dumpSinkHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	appId := appid.FromUrl(ws.Request().URL)
	authToken := ws.Request().Header.Get("Authorization")

	if code := httpServer.isAuthorized(appId, authToken, clientAddress); code > 200 {
		ws.CloseWithStatus(code)
		return
	}

	dumpChan := httpServer.messageRouter.registerDumpChan(appId)

	for message := range dumpChan {
		err := websocket.Message.Send(ws, message.GetRawMessage())
		if err != nil {
			httpServer.logger.Debugf("Dump Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", clientAddress, err)
		} else {
			httpServer.logger.Debugf("Dump Sink %s: Successfully sent data", clientAddress)
		}
	}

	ws.Close()
}

func contains(valueToFind string, values []string) bool {
	for _, value := range values {
		if valueToFind == value {
			return true
		}
	}
	return false
}
