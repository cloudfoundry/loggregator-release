package websocketserver

import (
	"doppler/sinks"
	"doppler/sinks/websocket"
	"doppler/sinkserver/sinkmanager"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/server"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	gorilla "github.com/gorilla/websocket"
)

type WebsocketServer struct {
	sinkManager       *sinkmanager.SinkManager
	keepAliveInterval time.Duration
	bufferSize        uint
	logger            *gosteno.Logger
	listener          net.Listener
	dropsondeOrigin   string

	done chan struct{}
}

func New(apiEndpoint string, sinkManager *sinkmanager.SinkManager, keepAliveInterval time.Duration, messageDrainBufferSize uint, dropsondeOrigin string, logger *gosteno.Logger) (*WebsocketServer, error) {
	logger.Infof("WebsocketServer: Listening for sinks at %s", apiEndpoint)

	listener, e := net.Listen("tcp", apiEndpoint)
	if e != nil {
		return nil, e
	}

	return &WebsocketServer{
		listener:          listener,
		sinkManager:       sinkManager,
		keepAliveInterval: keepAliveInterval,
		bufferSize:        messageDrainBufferSize,
		logger:            logger,
		dropsondeOrigin:   dropsondeOrigin,
		done:              make(chan struct{}),
	}, nil
}

func (w *WebsocketServer) Start() {
	s := &http.Server{Handler: w}
	err := s.Serve(w.listener)
	w.logger.Debugf("serve ended with %v", err.Error())
	close(w.done)
}

func (w *WebsocketServer) Stop() {
	w.logger.Debug("stopping websocket server")
	w.listener.Close()
	<-w.done
}

type wsHandler func(*gorilla.Conn)

func (w *WebsocketServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	w.logger.Debug("WebsocketServer.ServeHTTP: starting")
	var handler wsHandler
	var err error

	paths := strings.Split(request.URL.Path, "/")
	endpointName := paths[1]

	if endpointName == "firehose" {
		handler, err = w.firehoseHandler(paths, writer, request)
	} else {
		handler, err = w.appHandler(paths, writer, request)
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

	defer func() {
		ws.WriteControl(gorilla.CloseMessage, gorilla.FormatCloseMessage(gorilla.CloseNormalClosure, ""), time.Time{})
		ws.Close()
	}()

	handler(ws)
}

func (w *WebsocketServer) firehoseHandler(paths []string, writer http.ResponseWriter, request *http.Request) (wsHandler, error) {
	firehoseSubscriptionId := paths[2]
	f := func(ws *gorilla.Conn) {
		w.streamFirehose(firehoseSubscriptionId, ws)
	}
	return f, nil
}

// ^/apps/(.*)/(recentlogs|stream|containermetrics)$")
func (w *WebsocketServer) appHandler(paths []string, writer http.ResponseWriter, request *http.Request) (wsHandler, error) {
	var handler func(string, *gorilla.Conn)

	if len(paths) != 4 || paths[1] != "apps" {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", request.URL.Path)
		return nil, fmt.Errorf("Resource Not Found. %s", request.URL.Path)
	}
	appId := paths[2]

	if appId == "" {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(writer, "App ID missing. Make request to /apps/APP_ID/%s", paths[3])

		w.logInvalidApp(request.RemoteAddr)
		return nil, errors.New("Validation error (returning 400): No AppId")
	}
	endpoint := paths[3]

	switch endpoint {
	case "stream":
		handler = w.streamLogs
	case "recentlogs":
		handler = w.recentLogs
	case "containermetrics":
		handler = w.latestContainerMetrics
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

func (w *WebsocketServer) streamFirehose(subscriptionId string, websocketConnection *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting firehose wss sink")
	w.streamWebsocket(subscriptionId, websocketConnection, w.sinkManager.RegisterFirehoseSink, w.sinkManager.UnregisterFirehoseSink)
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

func (w *WebsocketServer) latestContainerMetrics(appId string, websocketConnection *gorilla.Conn) {
	metrics := w.sinkManager.LatestContainerMetrics(appId)
	sendMessagesToWebsocket(metrics, websocketConnection, w.logger)
}

func (w *WebsocketServer) logInvalidApp(address string) {
	message := fmt.Sprintf("WebsocketServer: Did not accept sink connection with invalid app id: %s.", address)
	w.logger.Warn(message)
}

func sendMessagesToWebsocket(envelopes []*events.Envelope, websocketConnection *gorilla.Conn, logger *gosteno.Logger) {
	for _, messageEnvelope := range envelopes {
		envelopeBytes, err := proto.Marshal(messageEnvelope)
		if err != nil {
			logger.Errorf("Websocket Server %s: Error marshalling %s envelope from origin %s: %s", websocketConnection.RemoteAddr(), messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), err.Error())
			continue
		}

		err = websocketConnection.WriteMessage(gorilla.BinaryMessage, envelopeBytes)
		if err != nil {
			logger.Debugf("Websocket Server %s: Error when trying to send data to sink %s. Requesting close. Err: %v", websocketConnection.RemoteAddr(), err)
		} else {
			logger.Debugf("Websocket Server %s: Successfully sent data", websocketConnection.RemoteAddr())
		}
	}
}
