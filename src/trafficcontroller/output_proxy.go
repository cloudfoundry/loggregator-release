package trafficcontroller

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"net/url"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/hasher"
)

type Proxy struct {
	host      string
	hashers   []*hasher.Hasher
	logger    *gosteno.Logger
	authorize authorization.LogAccessAuthorizer
	listener  net.Listener
}

type websocketHandler interface {
	HandleWebSocket(string, string, []*hasher.Hasher)
}

var NewProxyHandlerProvider func(*websocket.Conn, *gosteno.Logger) websocketHandler

func init() {
	NewProxyHandlerProvider = func(ws *websocket.Conn, logger *gosteno.Logger) websocketHandler {
		return NewProxyHandler(ws, logger)
	}
}

func NewProxy(host string, hashers []*hasher.Hasher, authorizer authorization.LogAccessAuthorizer, logger *gosteno.Logger) *Proxy {
	return &Proxy{host: host, hashers: hashers, authorize: authorizer, logger: logger}
}

func (proxy *Proxy) Start() error {
	l, err := net.Listen("tcp", proxy.host)
	if err != nil {
		return err
	}
	proxy.listener = l
	server := &http.Server{Addr: proxy.host, Handler: proxy}
	return server.Serve(proxy.listener)
}

func (proxy *Proxy) Stop() {
	proxy.listener.Close()
}

func (proxy *Proxy) isAuthorized(appId, authToken string, clientAddress string) (bool, *logmessage.LogMessage) {
	newLogMessage := func(message []byte) *logmessage.LogMessage {
		currentTime := time.Now()
		messageType := logmessage.LogMessage_ERR

		return &logmessage.LogMessage{
			Message:     message,
			AppId:       proto.String(appId),
			MessageType: &messageType,
			SourceName:  proto.String("LGR"),
			Timestamp:   proto.Int64(currentTime.UnixNano()),
		}
	}

	if appId == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection with invalid app id: %s.", clientAddress)
		proxy.logger.Warn(message)
		return false, newLogMessage([]byte("Error: Invalid target"))
	}

	if authToken == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection from %s without authorization.", clientAddress)
		proxy.logger.Warnf(message)
		return false, newLogMessage([]byte("Error: Authorization not provided"))
	}

	if !proxy.authorize(authToken, appId, proxy.logger) {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		proxy.logger.Warn(message)
		return false, newLogMessage([]byte("Error: Invalid authorization"))
	}

	return true, nil
}

func upgrade(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "Not a websocket handshake", 400)
		return nil
	}
	return ws
}

func (proxy *Proxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	clientAddress := r.RemoteAddr
	appId := r.Form.Get("app")

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		authToken = extractAuthTokenFromUrl(r.URL)
	}

	ws := upgrade(rw, r)
	defer ws.Close()
	authorized, errorMessage := proxy.isAuthorized(appId, authToken, clientAddress)
	if !authorized {
		data, err := proto.Marshal(errorMessage)
		if err != nil {
			proxy.logger.Errorf("Error marshalling log message: %s", err)
		}
		ws.WriteMessage(websocket.BinaryMessage, data)

		return
	}

	proxyHandler := NewProxyHandlerProvider(ws, proxy.logger)

	proxyHandler.HandleWebSocket(appId, r.URL.RequestURI(), proxy.hashers)
}

func extractAuthTokenFromUrl(u *url.URL) string {
	authorization := ""
	queryValues := u.Query()
	if len(queryValues["authorization"]) == 1 {
		authorization = queryValues["authorization"][0]
	}
	return authorization
}
