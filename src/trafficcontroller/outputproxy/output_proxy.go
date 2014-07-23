package outputproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
)

type Proxy struct {
	loggregatorServerProvider LoggregatorServerProvider
	logger                    *gosteno.Logger
	authorize                 authorization.LogAccessAuthorizer
	listener                  net.Listener
}

type WebsocketHandler interface {
	HandleWebSocket(string, string, []hasher.Hasher)
}

var NewWebsocketHandlerProvider = func(messages <-chan []byte) http.Handler {
	return handlers.NewWebsocketHandler(messages, 30*time.Second)
}

var NewHttpHandlerProvider = func(messages <-chan []byte) http.Handler {
	return handlers.NewHttpHandler(messages)
}

var NewWebsocketListener = func() listener.Listener {
	return listener.NewWebsocket()
}

type LoggregatorServerProvider interface {
	LoggregatorServersForAppId(appId string) []string
}

type hashingLoggregatorServerProvider struct {
	hashers []hasher.Hasher
}

func NewHashingLoggregatorServerProvider(hashers []hasher.Hasher) LoggregatorServerProvider {
	return &hashingLoggregatorServerProvider{hashers: hashers}
}

func (h *hashingLoggregatorServerProvider) LoggregatorServersForAppId(appId string) []string {
	result := []string{}
	for _, hasher := range h.hashers {
		result = append(result, hasher.GetLoggregatorServerForAppId(appId))
	}
	return result
}

func NewProxy(loggregatorServerProvider LoggregatorServerProvider, authorizer authorization.LogAccessAuthorizer, logger *gosteno.Logger) *Proxy {
	return &Proxy{loggregatorServerProvider: loggregatorServerProvider, authorize: authorizer, logger: logger}
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

func (proxy *Proxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	r.ParseForm()
	clientAddress := r.RemoteAddr
	appId := r.Form.Get("app")

	var loggregatorServerListenerCount sync.WaitGroup

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		authToken = extractAuthTokenFromUrl(r.URL)
	}

	authorized, errorMessage := proxy.isAuthorized(appId, authToken, clientAddress)
	if !authorized {
		rw.Header().Set("WWW-Authenticate", "Basic")
		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "You are not authorized. %s", errorMessage)
		return
	}

	messagesChan := make(chan []byte, 100)
	stopChan := make(chan struct{})
	defer close(stopChan)

	serverAddresses := proxy.loggregatorServerProvider.LoggregatorServersForAppId(appId)

	for _, serverAddress := range serverAddresses {
		serverUrlForAppId := fmt.Sprintf("ws://%s%s?app=%s", serverAddress, r.URL.Path, appId)
		loggregatorServerListenerCount.Add(1)
		l := NewWebsocketListener()
		go func() {
			err := l.Start(serverUrlForAppId, appId, messagesChan, stopChan)
			if err != nil {
				errorMsg := fmt.Sprintf("proxy: error connecting to a loggregator server")
				messagesChan <- generateLogMessage(errorMsg, appId)
				proxy.logger.Infof("proxy: error connecting %s %s %s", appId, r.URL.Path, err.Error())
			}
			loggregatorServerListenerCount.Done()
		}()
	}

	go func() {
		loggregatorServerListenerCount.Wait()
		close(messagesChan)
	}()

	var h http.Handler

	if recentViaHttp(r) {
		h = NewHttpHandlerProvider(messagesChan)
	} else {
		h = NewWebsocketHandlerProvider(messagesChan)
	}

	h.ServeHTTP(rw, r)
}

func extractAuthTokenFromUrl(u *url.URL) string {
	authorization := ""
	queryValues := u.Query()
	if len(queryValues["authorization"]) == 1 {
		authorization = queryValues["authorization"][0]
	}
	return authorization
}

func recentViaHttp(r *http.Request) bool {
	return r.URL.Path == "/recent"
}

func generateLogMessage(messageString string, appId string) []byte {
	messageType := logmessage.LogMessage_ERR
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: &messageType,
		SourceName:  proto.String("LGR"),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	msg, _ := proto.Marshal(logMessage)
	return msg
}
