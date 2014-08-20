package outputproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/listener"
)

var CheckLoggregatorServersInterval = 1 * time.Second
var WebsocketKeepAliveDuration = 30 * time.Second

type Proxy struct {
	cfcomponent.Component
	loggregatorServerProvider LoggregatorServerProvider
	logger                    *gosteno.Logger
	authorize                 authorization.LogAccessAuthorizer
	listener                  net.Listener
}

var NewWebsocketHandlerProvider = func(messages <-chan []byte) http.Handler {
	return handlers.NewWebsocketHandler(messages, WebsocketKeepAliveDuration)
}

var NewHttpHandlerProvider = func(messages <-chan []byte) http.Handler {
	return handlers.NewHttpHandler(messages)
}

var NewWebsocketListener = func() listener.Listener {
	return listener.NewWebsocket()
}

type LoggregatorServerProvider interface {
	LoggregatorServerAddresses() []string
}

type dynamicLoggregatorServerProvider struct {
	clientPool *clientpool.LoggregatorClientPool
}

func NewDynamicLoggregatorServerProvider(clientPool *clientpool.LoggregatorClientPool) LoggregatorServerProvider {
	return &dynamicLoggregatorServerProvider{clientPool}
}

func (p *dynamicLoggregatorServerProvider) LoggregatorServerAddresses() []string {
	return p.clientPool.ListAddresses()
}

func NewProxy(loggregatorServerProvider LoggregatorServerProvider, authorizer authorization.LogAccessAuthorizer, config cfcomponent.Config, logger *gosteno.Logger) *Proxy {
	var instrumentables []instrumentation.Instrumentable

	cfc, err := cfcomponent.NewComponent(
		logger,
		"LoggregatorTrafficcontroller",
		0,
		&TrafficControllerMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		instrumentables,
	)

	if err != nil {
		return nil
	}

	return &Proxy{Component: cfc, loggregatorServerProvider: loggregatorServerProvider, authorize: authorizer, logger: logger}
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

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		authToken = extractAuthTokenFromCookie(r.Cookies())
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

	go proxy.handleLoggregatorConnections(r, appId, messagesChan, stopChan)

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

func extractAuthTokenFromCookie(cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "authorization" {
			value, err := url.QueryUnescape(cookie.Value)
			if err != nil {
				return ""
			}

			return value
		}
	}

	return ""
}

func recentViaHttp(r *http.Request) bool {
	matched, _ := regexp.MatchString(`^/recent\b`, r.URL.Path)
	return matched
}

func recent(r *http.Request) bool {
	matched, _ := regexp.MatchString(`^/(recent|dump)\b`, r.URL.Path)
	return matched
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

type TrafficControllerMonitor struct {
}

func (hm TrafficControllerMonitor) Ok() bool {
	return true
}

func (proxy *Proxy) handleLoggregatorConnections(r *http.Request, appId string, messagesChan chan<- []byte, stopChan <-chan struct{}) {
	defer close(messagesChan)
	loggregatorConnections := &loggregatorConnections{}

	connectToNewServerAddresses := func(serverAddresses []string) {
		for _, serverAddress := range serverAddresses {
			if loggregatorConnections.alreadyConnectedToServer(serverAddress) {
				continue
			}
			loggregatorConnections.addConnectedServer(serverAddress)

			serverUrlForAppId := fmt.Sprintf("ws://%s%s?app=%s", serverAddress, r.URL.Path, appId)
			l := NewWebsocketListener()
			serverAddress := serverAddress
			go func() {
				err := l.Start(serverUrlForAppId, appId, messagesChan, stopChan)

				if err != nil {
					errorMsg := fmt.Sprintf("proxy: error connecting to a loggregator server")
					messagesChan <- generateLogMessage(errorMsg, appId)
					proxy.logger.Infof("proxy: error connecting %s %s %s", appId, r.URL.Path, err.Error())
				}
				loggregatorConnections.removeConnectedServer(serverAddress)
			}()
		}
	}

	checkLoggregatorServersTicker := time.NewTicker(CheckLoggregatorServersInterval)
	defer checkLoggregatorServersTicker.Stop()
loop:
	for {
		serverAddresses := proxy.loggregatorServerProvider.LoggregatorServerAddresses()
		connectToNewServerAddresses(serverAddresses)
		if recent(r) {
			break
		}
		select {
		case <-checkLoggregatorServersTicker.C:
		case <-stopChan:
			break loop
		}
	}

	loggregatorConnections.Wait()
}

type loggregatorConnections struct {
	connectedAddresses []string
	sync.Mutex
	sync.WaitGroup
}

func (l *loggregatorConnections) alreadyConnectedToServer(serverAddress string) bool {
	l.Lock()
	defer l.Unlock()

	for _, address := range l.connectedAddresses {
		if address == serverAddress {
			return true
		}
	}
	return false
}

func (l *loggregatorConnections) addConnectedServer(serverAddress string) {
	l.Lock()
	defer l.Unlock()

	l.Add(1)
	l.connectedAddresses = append(l.connectedAddresses, serverAddress)
}

func (l *loggregatorConnections) removeConnectedServer(serverAddress string) {
	l.Lock()
	defer l.Unlock()
	defer l.Done()

	for index, address := range l.connectedAddresses {
		if address == serverAddress {
			l.connectedAddresses = append(l.connectedAddresses[0:index],
				l.connectedAddresses[index+1:]...)
			return
		}
	}
}
