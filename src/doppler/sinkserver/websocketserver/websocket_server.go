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

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/server"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	gorilla "github.com/gorilla/websocket"
)

type Batcher interface {
	BatchIncrementCounter(name string)
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type envelopeCounter struct {
	endpoint string
	batcher  Batcher
}

func (c *envelopeCounter) Increment(typ events.Envelope_EventType) {
	c.batcher.BatchCounter("sentEnvelopes").
		SetTag("protocol", "ws").
		SetTag("event_type", typ.String()).
		SetTag("endpoint", c.endpoint).
		Increment()
}

func newStreamCounter(batcher Batcher) *envelopeCounter {
	return &envelopeCounter{
		endpoint: "stream",
		batcher:  batcher,
	}
}

type firehoseCounter struct {
	envelopeCounter
	subscriptionID string
}

func newFirehoseCounter(subscriptionID string, batcher Batcher) *firehoseCounter {
	counter := &firehoseCounter{}
	counter.endpoint = "firehose"
	counter.subscriptionID = subscriptionID
	counter.batcher = batcher
	return counter
}

func (f *firehoseCounter) Increment(typ events.Envelope_EventType) {
	f.batcher.BatchCounter("sentMessagesFirehose").
		SetTag("subscription_id", f.subscriptionID).
		Increment()

	f.envelopeCounter.Increment(typ)
}

type WebsocketServer struct {
	sinkManager       *sinkmanager.SinkManager
	writeTimeout      time.Duration
	keepAliveInterval time.Duration
	bufferSize        uint
	batcher           Batcher
	logger            *gosteno.Logger
	listener          net.Listener
	dropsondeOrigin   string

	done chan struct{}
}

func New(apiEndpoint string, sinkManager *sinkmanager.SinkManager, writeTimeout time.Duration, keepAliveInterval time.Duration, messageDrainBufferSize uint, dropsondeOrigin string, batcher Batcher, logger *gosteno.Logger) (*WebsocketServer, error) {
	logger.Infof("WebsocketServer: Listening for sinks at %s", apiEndpoint)

	listener, e := net.Listen("tcp", apiEndpoint)
	if e != nil {
		return nil, e
	}

	return &WebsocketServer{
		listener:          listener,
		sinkManager:       sinkManager,
		writeTimeout:      writeTimeout,
		keepAliveInterval: keepAliveInterval,
		bufferSize:        messageDrainBufferSize,
		batcher:           batcher,
		logger:            logger,
		dropsondeOrigin:   dropsondeOrigin,
		done:              make(chan struct{}),
	}, nil
}

func (w *WebsocketServer) Start() {
	s := &http.Server{Handler: w}
	err := s.Serve(w.listener)
	w.logger.Errorf("serve ended with %v", err.Error())
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
		w.logger.Errorf("WebsocketServer.ServeHTTP: Upgrade error (returning 400): %s", err.Error())
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
	if len(paths) != 3 {
		http.Error(writer, "missing subscription id in firehose request: "+request.URL.Path, http.StatusBadRequest)
		return nil, fmt.Errorf("missing subscription id in firehose request: (returning %d) %s", http.StatusBadRequest, request.URL.Path)
	}
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
		fmt.Fprint(writer, "Resource Not Found.")
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
		http.Error(writer, "Invalid path", 400)
		return nil, fmt.Errorf("Invalid path (returning 400): invalid path %s", request.URL.Path)
	}

	f := func(ws *gorilla.Conn) {
		handler(appId, ws)
	}
	return f, nil
}

func (w *WebsocketServer) streamLogs(appId string, websocketConnection *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting a wss sink for app %s", appId)
	websocketSink := websocket.NewWebsocketSink(
		appId,
		w.logger,
		websocketConnection,
		w.bufferSize,
		w.writeTimeout,
		w.dropsondeOrigin,
	)

	websocketSink.SetCounter(newStreamCounter(w.batcher))

	w.streamWebsocket(websocketSink, websocketConnection, w.sinkManager.RegisterSink, w.sinkManager.UnregisterSink)
}

func (w *WebsocketServer) streamFirehose(subscriptionId string, websocketConnection *gorilla.Conn) {
	w.logger.Debugf("WebsocketServer: Requesting firehose wss sink")
	websocketSink := websocket.NewWebsocketSink(
		subscriptionId,
		w.logger,
		websocketConnection,
		w.bufferSize,
		w.writeTimeout,
		w.dropsondeOrigin,
	)

	firehoseCounter := newFirehoseCounter(subscriptionId, w.batcher)
	websocketSink.SetCounter(firehoseCounter)

	w.streamWebsocket(websocketSink, websocketConnection, w.sinkManager.RegisterFirehoseSink, w.sinkManager.UnregisterFirehoseSink)
}

func (w *WebsocketServer) streamWebsocket(websocketSink *websocket.WebsocketSink, websocketConnection *gorilla.Conn, register func(sinks.Sink) bool, unregister func(sinks.Sink)) {
	register(websocketSink)
	defer unregister(websocketSink)

	go websocketConnection.ReadMessage()
	server.NewKeepAlive(websocketConnection, w.keepAliveInterval).Run()
}

func (w *WebsocketServer) recentLogs(appId string, websocketConnection *gorilla.Conn) {
	logMessages := w.sinkManager.RecentLogsFor(appId)
	sendMessagesToWebsocket("recentlogs", logMessages, websocketConnection, w.batcher, w.logger)
}

func (w *WebsocketServer) latestContainerMetrics(appId string, websocketConnection *gorilla.Conn) {
	metrics := w.sinkManager.LatestContainerMetrics(appId)
	sendMessagesToWebsocket("containermetrics", metrics, websocketConnection, w.batcher, w.logger)
}

func (w *WebsocketServer) logInvalidApp(address string) {
	message := fmt.Sprintf("WebsocketServer: Did not accept sink connection with invalid app id: %s.", address)
	w.logger.Warn(message)
}

func sendMessagesToWebsocket(endpoint string, envelopes []*events.Envelope, websocketConnection *gorilla.Conn, batcher Batcher, logger *gosteno.Logger) {
	for _, messageEnvelope := range envelopes {
		envelopeBytes, err := proto.Marshal(messageEnvelope)
		if err != nil {
			logger.Errorf("Websocket Server %s: Error marshalling %s envelope from origin %s: %s", websocketConnection.RemoteAddr(), messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), err.Error())
			continue
		}

		err = websocketConnection.WriteMessage(gorilla.BinaryMessage, envelopeBytes)
		if err != nil {
			logger.Errorf("Websocket Server %s: Error when trying to send data to sink %s. Err: %v", websocketConnection.RemoteAddr(), err)
			continue
		}
		batcher.BatchCounter("sentEnvelopes").
			SetTag("protocol", "ws").
			SetTag("event_type", messageEnvelope.GetEventType().String()).
			SetTag("endpoint", endpoint).
			Increment()

		logger.Debugf("Websocket Server %s: Successfully sent data", websocketConnection.RemoteAddr())
	}
}
