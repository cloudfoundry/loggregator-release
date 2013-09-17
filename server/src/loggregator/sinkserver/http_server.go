package sinkserver

import (
	"bytes"
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/authorization"
	"loggregator/sinks"
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
	http.HandleFunc(DUMP_PATH, httpServer.dumpSinkHandler)
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

func (httpServer *httpServer) websocketSinkHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()

	appId := appid.FromUrl(ws.Request().URL)
	authToken := ws.Request().Header.Get("Authorization")

	if appId == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection with invalid app id: %s.", clientAddress)
		httpServer.logger.Warn(message)
		ws.CloseWithStatus(4000)
		return
	}

	if authToken == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection from %s without authorization.", clientAddress)
		httpServer.logger.Warnf(message)
		ws.CloseWithStatus(4001)
		return
	}

	if !httpServer.authorize(authToken, appId, httpServer.logger) {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		httpServer.logger.Warn(message)
		ws.CloseWithStatus(4002)
		return
	}

	defer ws.Close()
	s := sinks.NewWebsocketSink(appId, httpServer.logger, ws, clientAddress, httpServer.keepAliveInterval)
	httpServer.logger.Debugf("HttpServer: Requesting a wss sink for app %s", s.AppId())
	httpServer.messageRouter.sinkOpenChan <- s

	s.Run(httpServer.messageRouter.sinkCloseChan)
}

func (httpServer *httpServer) dumpSinkHandler(rw http.ResponseWriter, req *http.Request) {
	appId := appid.FromUrl(req.URL)
	authToken := req.Header.Get("Authorization")

	if appId == "" {
		httpServer.logger.Warn("HttpServer: Did not accept dump connection with invalid app id.")
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if !httpServer.authorize(authToken, appId, httpServer.logger) {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access target [%s].", authToken, appId)
		httpServer.logger.Warn(message)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	buffer := bytes.NewBufferString("")
	dumpChan := make(chan logmessage.Message)
	dumpReceiver := dumpReceiver{appId: appId, outputChannel: dumpChan}
	httpServer.messageRouter.dumpReceiverChan <- dumpReceiver

	for message := range dumpChan {
		if message.GetRawMessageLength() > 0 {
			logmessage.DumpMessage(message, buffer)
		}
	}

	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.Write(buffer.Bytes())
}

func contains(valueToFind string, values []string) bool {
	for _, value := range values {
		if valueToFind == value {
			return true
		}
	}
	return false
}
