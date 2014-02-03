package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinks"
	"net"
	"net/http"
	"time"
)

const (
	TAIL_LOGS_PATH   = "/tail/"
	RECENT_LOGS_PATH = "/dump/"
)

type websocketServer struct {
	apiEndpoint       string
	sinkManager       *SinkManager
	keepAliveInterval time.Duration
	bufferSize        uint
	logger            *gosteno.Logger
}

func NewWebsocketServer(apiEndpoint string, sinkManager *SinkManager, keepAliveInterval time.Duration, wSMessageBufferSize uint, logger *gosteno.Logger) *websocketServer {
	return &websocketServer{
		apiEndpoint:       apiEndpoint,
		sinkManager:       sinkManager,
		keepAliveInterval: keepAliveInterval,
		bufferSize:        wSMessageBufferSize,
		logger:            logger,
	}
}

func (websocketServer *websocketServer) Start() {
	websocketServer.logger.Infof("WebsocketServer: Listening for sinks at %s", websocketServer.apiEndpoint)
	if err := http.ListenAndServe(websocketServer.apiEndpoint, websocket.Handler(websocketServer.route)); err != nil {
		panic(err)
	}
}

func (websocketServer *websocketServer) route(ws *websocket.Conn) {
	switch ws.Request().URL.Path {
	case TAIL_LOGS_PATH:
		websocketServer.streamLogs(ws)
	case RECENT_LOGS_PATH:
		websocketServer.recentLogs(ws)
	default:
		ws.CloseWithStatus(400)
		return
	}
}

func (websocketServer *websocketServer) streamLogs(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	appId := appid.FromUrl(ws.Request().URL)

	if appId == "" {
		websocketServer.logInvalidApp(clientAddress)
		ws.CloseWithStatus(4000)
		return
	}

	websocketSink := sinks.NewWebsocketSink(
		appId,
		websocketServer.logger,
		ws,
		clientAddress,
		websocketServer.sinkManager.sinkCloseChan,
		websocketServer.keepAliveInterval,
		websocketServer.bufferSize,
	)
	websocketServer.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", websocketSink.AppId())
	websocketServer.sinkManager.sinkOpenChan <- websocketSink

	websocketSink.Run()
}

func (websocketServer *websocketServer) recentLogs(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	appId := appid.FromUrl(ws.Request().URL)

	if appId == "" {
		websocketServer.logInvalidApp(clientAddress)
		ws.CloseWithStatus(4000)
		return
	}

	logMessages := websocketServer.sinkManager.recentLogsFor(appId)

	sendMessagesToWebsocket(logMessages, ws, clientAddress, websocketServer.logger)

	ws.Close()
}

func (websocketServer *websocketServer) logInvalidApp(address net.Addr) {
	message := fmt.Sprintf("websocketServer: Did not accept sink connection with invalid app id: %s.", address)
	websocketServer.logger.Warn(message)
}

func sendMessagesToWebsocket(logMessages []*logmessage.Message, ws *websocket.Conn, clientAddress net.Addr, logger *gosteno.Logger) {
	for _, message := range logMessages {
		err := websocket.Message.Send(ws, message.GetRawMessage())
		if err != nil {
			logger.Debugf("Dump Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", clientAddress, err)
		} else {
			logger.Debugf("Dump Sink %s: Successfully sent data", clientAddress)
		}
	}
}
