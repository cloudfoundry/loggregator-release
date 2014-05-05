package websocketserver

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	gorilla "github.com/gorilla/websocket"
	"loggregator/sinks/websocket"
	"loggregator/sinkserver/sinkmanager"
	"net"
	"net/http"
	"sync"
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
	sync.RWMutex
}

func New(apiEndpoint string, sinkManager *sinkmanager.SinkManager, keepAliveInterval time.Duration, wSMessageBufferSize uint, logger *gosteno.Logger) *WebsocketServer {
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

	w.Lock()
	w.listener = listener
	w.Unlock()

	server := &http.Server{Addr: w.apiEndpoint, Handler: w}
	server.Serve(w.listener)
}

func (w *WebsocketServer) Stop() {
	w.Lock()
	defer w.Unlock()
	w.listener.Close()
}

func (w *WebsocketServer) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	var handler func(string, *gorilla.Conn)

	switch r.URL.Path {
	case TAIL_LOGS_PATH:
		handler = w.streamLogs
	case RECENT_LOGS_PATH:
		handler = w.recentLogs
	default:
		http.Error(rw, "invalid path "+r.URL.Path, 400)
		return
	}

	appId, err := w.validate(r)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	ws, err := gorilla.Upgrade(rw, r, nil, 1024, 1024)
	if err != nil {
		http.Error(rw, err.Error(), 400)
		return
	}

	defer ws.Close()
	defer ws.WriteControl(gorilla.CloseMessage, []byte{}, time.Time{})

	handler(appId, ws)
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

func (w *WebsocketServer) streamLogs(appId string, ws *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", appId)
	websocketSink := websocket.NewWebsocketSink(
		appId,
		w.logger,
		ws,
		w.bufferSize,
	)

	w.sinkManager.RegisterSink(websocketSink)
	defer w.sinkManager.UnregisterSink(websocketSink)

	w.waitForKeepAliveTimeout(ws)
}

func (w *WebsocketServer) recentLogs(appId string, ws *gorilla.Conn) {
	logMessages := w.sinkManager.RecentLogsFor(appId)
	sendMessagesToWebsocket(logMessages, ws, w.logger)
}

func (w *WebsocketServer) logInvalidApp(address string) {
	message := fmt.Sprintf("WebsocketServer: Did not accept sink connection with invalid app id: %s.", address)
	w.logger.Warn(message)
}

func sendMessagesToWebsocket(logMessages []*logmessage.Message, ws *gorilla.Conn, logger *gosteno.Logger) {
	for _, message := range logMessages {
		err := ws.WriteMessage(gorilla.BinaryMessage, message.GetRawMessage())
		if err != nil {
			logger.Debugf("Dump Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", ws.RemoteAddr(), err)
		} else {
			logger.Debugf("Dump Sink %s: Successfully sent data", ws.RemoteAddr())
		}
	}
}

func (w *WebsocketServer) waitForKeepAliveTimeout(ws *gorilla.Conn) {
	ws.SetPongHandler(func(message string) error {
		w.logger.Debugf("Websocket Sink %s: Got pong %s, extending deadline\n", ws.RemoteAddr(), message)
		ws.SetReadDeadline(time.Now().Add(w.keepAliveInterval))
		return nil
	})

	go emitPings(ws, w.keepAliveInterval/2, w.logger)

	_, _, err := ws.ReadMessage()
	if err != nil {
		w.logger.Debugf("Websocket Sink %s: Timeout waiting for pong. Stopping listening\n", ws.RemoteAddr())
		return
	}
}

func emitPings(ws *gorilla.Conn, interval time.Duration, logger *gosteno.Logger) {
	for {
		err := ws.WriteControl(gorilla.PingMessage, []byte{42}, time.Time{})
		if err != nil {
			logger.Debugf("Websocket Sink %s: Error sending ping: %v\n", ws.RemoteAddr(), err)
			return
		}

		time.Sleep(interval)
	}
}
