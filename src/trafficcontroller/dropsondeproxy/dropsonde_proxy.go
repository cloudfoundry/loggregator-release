package dropsondeproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/listener"
)

var WebsocketKeepAliveDuration = 30 * time.Second

type Proxy struct {
	cfcomponent.Component
	logger    *gosteno.Logger
	authorize authorization.LogAccessAuthorizer
	listener  net.Listener
}

var NewWebsocketHandlerProvider = func(messages <-chan []byte) http.Handler {
	return handlers.NewWebsocketHandler(messages, WebsocketKeepAliveDuration)
}

var NewHttpHandlerProvider = func(messages chan []byte) http.Handler {
	return handlers.NewHttpHandler(messages)
}

var NewWebsocketListener = func() listener.Listener {
	return listener.NewWebsocket()
}

func NewDropsondeProxy(authorizer authorization.LogAccessAuthorizer, config cfcomponent.Config, logger *gosteno.Logger) *Proxy {
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

	return &Proxy{Component: cfc, authorize: authorizer, logger: logger}
}

func (proxy *Proxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	r.ParseForm()
	clientAddress := r.RemoteAddr
	re := regexp.MustCompile("^/apps/(.*)/(tailinglogs|recentlogs|stream)$")
	matches := re.FindStringSubmatch(r.URL.Path)
	if len(matches) != 3 {
		rw.Header().Set("WWW-Authenticate", "Basic")
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, "Resource Not Found. %s", r.URL.Path)
		return
	}
	appId := matches[1]
	endpoint := matches[2]

	authToken := r.Header.Get("Authorization")

	authorized, errorMessage := proxy.isAuthorized(appId, authToken, clientAddress)
	if !authorized {
		rw.Header().Set("WWW-Authenticate", "Basic")
		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "You are not authorized. %s", errorMessage)
		return
	}

	messagesChan := make(chan []byte, 100)

	var h http.Handler
	switch endpoint {
	case "tailinglogs":
		h = NewWebsocketHandlerProvider(messagesChan)
	case "recentlogs":
		h = NewHttpHandlerProvider(messagesChan)
	case "stream":
		h = NewWebsocketHandlerProvider(messagesChan)
	}

	h.ServeHTTP(rw, r)
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

func extractAuthTokenFromUrl(u *url.URL) string {
	authorization := ""
	queryValues := u.Query()
	if len(queryValues["authorization"]) == 1 {
		authorization = queryValues["authorization"][0]
	}
	return authorization
}

func recent(r *http.Request) bool {
	matched, _ := regexp.MatchString(`^/recentlogs\b`, r.URL.Path)
	return matched
}

type TrafficControllerMonitor struct {
}

func (hm TrafficControllerMonitor) Ok() bool {
	return true
}
