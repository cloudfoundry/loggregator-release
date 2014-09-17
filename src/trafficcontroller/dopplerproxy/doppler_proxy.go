package dopplerproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net/http"
	"net/url"
	"regexp"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/channel_group_connector"
)

var WebsocketKeepAliveDuration = 30 * time.Second

type Proxy struct {
	authorize       authorization.LogAccessAuthorizer
	handlerProvider HandlerProvider
	connector       channel_group_connector.ChannelGroupConnector
	logger          *gosteno.Logger
	cfcomponent.Component
}

type HandlerProvider func(string, <-chan []byte) http.Handler

func DefaultHandlerProvider(endpoint string, messages <-chan []byte) http.Handler {
	switch endpoint {
	case "recentlogs":
		return handlers.NewHttpHandler(messages)
	case "stream":
		fallthrough
	default:
		return handlers.NewWebsocketHandler(messages, WebsocketKeepAliveDuration)
	}
}

func NewDopplerProxy(authorizer authorization.LogAccessAuthorizer, handlerProvider HandlerProvider, connector channel_group_connector.ChannelGroupConnector, config cfcomponent.Config, logger *gosteno.Logger) *Proxy {
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

	return &Proxy{
		Component:       cfc,
		authorize:       authorizer,
		handlerProvider: handlerProvider,
		connector:       connector,
		logger:          logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	if req.Method == "HEAD" {
		return
	}

	clientAddress := req.RemoteAddr
	validPaths := regexp.MustCompile("^/apps/(.*)/(recentlogs|stream)$")
	matches := validPaths.FindStringSubmatch(req.URL.Path)
	if len(matches) != 3 {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", req.URL.Path)
		return
	}
	appId := matches[1]

	if appId == "" {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "App ID missing. Make request to /apps/APP_ID/%s", matches[2])
		return
	}
	endpoint := matches[2]

	authToken := getAuthToken(req)

	authorized, errorMessage := proxy.isAuthorized(appId, authToken, clientAddress)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", errorMessage.GetMessage())
		return
	}

	messagesChan := make(chan []byte, 100)
	stopChan := make(chan struct{})
	defer close(stopChan)

	reconnect := endpoint != "recentlogs"

	go proxy.connector.Connect("/"+endpoint, appId, messagesChan, stopChan, reconnect)

	handler := proxy.handlerProvider(endpoint, messagesChan)
	handler.ServeHTTP(writer, req)
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

func getAuthToken(req *http.Request) string {
	authToken := req.Header.Get("Authorization")

	if authToken == "" {
		authToken = extractAuthTokenFromCookie(req.Cookies())
	}

	return authToken
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

type TrafficControllerMonitor struct {
}

func (hm TrafficControllerMonitor) Ok() bool {
	return true
}
