package websocketserver

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/sinks"
	"doppler/sinks/websocket"
	"doppler/sinkserver/sinkmanager"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/server"
	gorilla "github.com/gorilla/websocket"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	STREAM_LOGS_PATH = "/stream"
	RECENT_LOGS_PATH = "/recent"
	FIREHOSE_PATH    = "/firehose"
)

type WebsocketServer struct {
	apiEndpoint       string
	sinkManager       *sinkmanager.SinkManager
	keepAliveInterval time.Duration
	bufferSize        uint
	logger            *gosteno.Logger
	listener          net.Listener
	dropsondeOrigin   string
	sync.RWMutex
}

func New(apiEndpoint string, sinkManager *sinkmanager.SinkManager, keepAliveInterval time.Duration, wSMessageBufferSize uint, logger *gosteno.Logger) *WebsocketServer {
	return &WebsocketServer{
		apiEndpoint:       apiEndpoint,
		sinkManager:       sinkManager,
		keepAliveInterval: keepAliveInterval,
		bufferSize:        wSMessageBufferSize,
		logger:            logger,
		dropsondeOrigin:   sinkManager.DropsondeOrigin,
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

	s := &http.Server{Addr: w.apiEndpoint, Handler: w}
	err := s.Serve(w.listener)
	w.logger.Debugf("serve ended with %v", err)
}

func (w *WebsocketServer) Stop() {
	w.Lock()
	defer w.Unlock()
	w.logger.Debug("stopping websocket server")
	w.listener.Close()
}

type wsHandler func(*gorilla.Conn)

func (w *WebsocketServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	w.logger.Debug("WebsocketServer.ServeHTTP: starting")
	var handler wsHandler
	var err error

	if request.URL.Path == FIREHOSE_PATH {
		handler, err = w.firehoseHandler(writer, request)
	} else {
		handler, err = w.appHandler(writer, request)
	}

	if err != nil {
		w.logger.Errorf("WebsocketServer.ServeHTTP: %s", err.Error())
		return
	}

	ws, err := gorilla.Upgrade(writer, request, nil, 1024, 1024)
	if err != nil {
		w.logger.Debugf("WebsocketServer.ServeHTTP: Upgrade error (returning 400): %s", err.Error())
		http.Error(writer, err.Error(), 400)
		return
	}

	defer ws.Close()
	defer ws.WriteControl(gorilla.CloseMessage, gorilla.FormatCloseMessage(gorilla.CloseNormalClosure, ""), time.Time{})

	handler(ws)
}

func (w *WebsocketServer) firehoseHandler(writer http.ResponseWriter, request *http.Request) (wsHandler, error) {
	return w.streamFirehose, nil
}

func (w *WebsocketServer) appHandler(writer http.ResponseWriter, request *http.Request) (wsHandler, error) {
	var handler func(string, *gorilla.Conn)

	validPaths := regexp.MustCompile("^/apps/(.*)/(recentlogs|stream)$")
	matches := validPaths.FindStringSubmatch(request.URL.Path)
	if len(matches) != 3 {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", request.URL.Path)
		return nil, fmt.Errorf("Resource Not Found. %s", request.URL.Path)
	}
	appId := matches[1]

	if appId == "" {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, "App ID missing. Make request to /apps/APP_ID/%s", matches[2])

		w.logInvalidApp(request.RemoteAddr)
		return nil, errors.New("Validation error (returning 400): No AppId")
	}
	endpoint := matches[2]

	switch endpoint {
	case "stream":
		handler = w.streamLogs
	case "recentlogs":
		handler = w.recentLogs
	default:
		http.Error(writer, "invalid path "+request.URL.Path, 400)
		return nil, fmt.Errorf("Invalid path (returning 400): invalid path %s", request.URL.Path)
	}

	f := func(ws *gorilla.Conn) {
		handler(appId, ws)
	}
	return f, nil
}

func (w *WebsocketServer) streamLogs(appId string, websocketConnection *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", appId)
	w.streamWebsocket(appId, websocketConnection, w.sinkManager.RegisterSink, w.sinkManager.UnregisterSink)
}

func (w *WebsocketServer) streamFirehose(websocketConnection *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting firehose wss sink")
	w.streamWebsocket(websocket.FIREHOSE_APP_ID, websocketConnection, w.sinkManager.RegisterFirehoseSink, w.sinkManager.UnregisterFirehoseSink)
}

func (w *WebsocketServer) streamWebsocket(appId string, websocketConnection *gorilla.Conn, register func(sinks.Sink) bool, unregister func(sinks.Sink)) {
	websocketSink := websocket.NewWebsocketSink(
		appId,
		w.logger,
		websocketConnection,
		w.bufferSize,
		w.dropsondeOrigin,
	)

	register(websocketSink)
	defer unregister(websocketSink)

	go websocketConnection.ReadMessage()
	server.NewKeepAlive(websocketConnection, w.keepAliveInterval).Run()
}

func (w *WebsocketServer) recentLogs(appId string, websocketConnection *gorilla.Conn) {
	logMessages := w.sinkManager.RecentLogsFor(appId)
	sendMessagesToWebsocket(logMessages, websocketConnection, w.logger)
}

func (w *WebsocketServer) logInvalidApp(address string) {
	message := fmt.Sprintf("WebsocketServer: Did not accept sink connection with invalid app id: %s.", address)
	w.logger.Warn(message)
}

func sendMessagesToWebsocket(logMessages []*events.Envelope, websocketConnection *gorilla.Conn, logger *gosteno.Logger) {
	for _, messageEnvelope := range logMessages {
		envelopeBytes, err := proto.Marshal(messageEnvelope)

		if err != nil {
			logger.Errorf("Websocket Server %s: Error marshalling %s envelope from origin %s: %s", websocketConnection.RemoteAddr(), messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), err.Error())
		}

		err = websocketConnection.WriteMessage(gorilla.BinaryMessage, envelopeBytes)
		if err != nil {
			logger.Debugf("Websocket Server %s: Error when trying to send data to sink %s. Requesting close. Err: %v", websocketConnection.RemoteAddr(), err)
		} else {
			logger.Debugf("Websocket Server %s: Successfully sent data", websocketConnection.RemoteAddr())
		}
	}
}
