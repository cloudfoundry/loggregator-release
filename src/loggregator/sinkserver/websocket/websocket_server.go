package websocket

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"loggregator/sinks"
	"loggregator/sinkserver/sinkmanager"
	"net"
	"net/http"
	"time"
)

const (
	TAIL_LOGS_PATH   = "/tail/"
	RECENT_LOGS_PATH = "/dump/"
)

type WebsocketServer struct {
	apiEndpoint       string
	sinkManager       *sinkmanager.SinkManager
	keepAliveInterval time.Duration
	bufferSize        uint
	logger            *gosteno.Logger
	listener          net.Listener
}

func NewWebsocketServer(apiEndpoint string, sinkManager *sinkmanager.SinkManager, keepAliveInterval time.Duration, wSMessageBufferSize uint, logger *gosteno.Logger) *WebsocketServer {
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

	listener, e := net.Listen("tcp", w.apiEndpoint)
	if e != nil {
		panic(e)
	}
	w.listener = listener

	server := &http.Server{Addr: w.apiEndpoint, Handler: w}
	server.Serve(w.listener)
}

func (w *WebsocketServer) Stop() {
	w.listener.Close()
}

func (w *WebsocketServer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	appId, err := w.validate(r)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}
	ws, err := websocket.Upgrade(rw, r, nil, 1024, 1024)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}
	defer ws.Close()
	switch r.URL.Path {
	case TAIL_LOGS_PATH:
		w.streamLogs(appId, ws)
	case RECENT_LOGS_PATH:
		w.recentLogs(appId, ws)
	default:
		http.Error(rw, err.Error(), 400)
		return
	}
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

func (w *WebsocketServer) streamLogs(appId string, ws *websocket.Conn) {
	defer ws.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})
	websocketSink := sinks.NewWebsocketSink(
		appId,
		w.logger,
		ws,
		w.keepAliveInterval,
		w.bufferSize,
	)
	w.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", websocketSink.AppId())
	w.sinkManager.RegisterSink(websocketSink, false)
}

func (w *WebsocketServer) recentLogs(appId string, ws *websocket.Conn) {
	defer ws.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})

	logMessages := w.sinkManager.RecentLogsFor(appId)
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
