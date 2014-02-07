package sinkserver

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"loggregator/sinks"
	"net/http"
	"time"
)

const (
	TAIL_LOGS_PATH   = "/tail/"
	RECENT_LOGS_PATH = "/dump/"
)

type WebsocketServer struct {
	apiEndpoint       string
	sinkManager       *SinkManager
	keepAliveInterval time.Duration
	bufferSize        uint
	logger            *gosteno.Logger
}

func NewWebsocketServer(apiEndpoint string, sinkManager *SinkManager, keepAliveInterval time.Duration, wSMessageBufferSize uint, logger *gosteno.Logger) *WebsocketServer {
	return &WebsocketServer{
		apiEndpoint:       apiEndpoint,
		sinkManager:       sinkManager,
		keepAliveInterval: keepAliveInterval,
		bufferSize:        wSMessageBufferSize,
		logger:            logger,
	}
}

func (w *WebsocketServer) Start() {
	w.logger.Infof("WebsocketServer: Listening for sinks at %s", w.apiEndpoint)
	http.HandleFunc(TAIL_LOGS_PATH, w.handleTail)
	http.HandleFunc(RECENT_LOGS_PATH, w.handleRecent)
	if err := http.ListenAndServe(w.apiEndpoint, nil); err != nil {
		panic(err)
	}
}

func (w *WebsocketServer) Stop() {
}

func upgrade(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "Not a websocket handshake", 400)
		return nil
	}
	return ws
}

func (w *WebsocketServer) validate(r *http.Request) (string, error) {
	appId := appid.FromUrl(r.URL)
	clientAddress := r.RemoteAddr
	if appId == "" {
		w.logInvalidApp(clientAddress)
		return "", errors.New("Invalid AppId")
	}
	return appId, nil
}

func (w *WebsocketServer) handleTail(rw http.ResponseWriter, r *http.Request) {
	appId, err := w.validate(r)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	w.streamLogs(appId, upgrade(rw, r))
}

func (w *WebsocketServer) handleRecent(rw http.ResponseWriter, r *http.Request) {
	appId, err := w.validate(r)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	w.recentLogs(appId, upgrade(rw, r))
}

func (w *WebsocketServer) streamLogs(appId string, ws *websocket.Conn) {

	websocketSink := sinks.NewWebsocketSink(
		appId,
		w.logger,
		ws,
		w.sinkManager.sinkCloseChan,
		w.keepAliveInterval,
		w.bufferSize,
	)
	w.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", websocketSink.AppId())
	w.sinkManager.sinkOpenChan <- websocketSink

	websocketSink.Run()
}

func (w *WebsocketServer) recentLogs(appId string, ws *websocket.Conn) {
	defer ws.Close()

	logMessages := w.sinkManager.recentLogsFor(appId)
	sendMessagesToWebsocket(logMessages, ws, w.logger)
}

func (w *WebsocketServer) logInvalidApp(address string) {
	message := fmt.Sprintf("WebsocketServer: Did not accept sink connection with invalid app id: %s.", address)
	w.logger.Warn(message)
}

func sendMessagesToWebsocket(logMessages []*logmessage.Message, ws *websocket.Conn, logger *gosteno.Logger) {
	for _, message := range logMessages {
		err := ws.WriteMessage(websocket.BinaryMessage, message.GetRawMessage())
		if err != nil {
			logger.Debugf("Dump Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", ws.RemoteAddr(), err)
		} else {
			logger.Debugf("Dump Sink %s: Successfully sent data", ws.RemoteAddr())
		}
	}
}
