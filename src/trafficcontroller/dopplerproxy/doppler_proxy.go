package dopplerproxy

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/http"
	"net/url"
	"regexp"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/doppler_endpoint"
)

const FIREHOSE_ID = "firehose"

type Proxy struct {
	logAuthorize   authorization.LogAccessAuthorizer
	adminAuthorize authorization.AdminAccessAuthorizer
	connector      channel_group_connector.ChannelGroupConnector
	translate      RequestTranslator
	logger         *gosteno.Logger
	cfcomponent.Component
}

type RequestTranslator func(request *http.Request) (*http.Request, error)

type Authorizer func(appId, authToken string, logger *gosteno.Logger) bool

func NewDopplerProxy(logAuthorize authorization.LogAccessAuthorizer, adminAuthorizer authorization.AdminAccessAuthorizer, connector channel_group_connector.ChannelGroupConnector, config cfcomponent.Config, translator RequestTranslator, logger *gosteno.Logger) *Proxy {
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
		Component:      cfc,
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		connector:      connector,
		translate:      translator,
		logger:         logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	proxy.logger.Debugf("doppler proxy: ServeHTTP entered with request %v", request)
	defer proxy.logger.Debugf("doppler proxy: ServeHTTP exited")

	translatedRequest, err := proxy.translate(request)
	if err != nil {
		proxy.logger.Errorf("DopplerProxy.ServeHTTP: unable to translate request: %s", err.Error())
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", request.URL.Path)
		return
	}

	if request.Method == "HEAD" {
		return
	}

	isFirehosePath, _ := regexp.MatchString(FIREHOSE_ID, translatedRequest.URL.Path)
	if isFirehosePath {
		proxy.serveFirehose(writer, translatedRequest)
	} else {
		proxy.serveAppLogs(writer, translatedRequest)
	}
}

func (proxy *Proxy) serveFirehose(writer http.ResponseWriter, request *http.Request) {
	clientAddress := request.RemoteAddr
	authToken := getAuthToken(request)

	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(FIREHOSE_ID, FIREHOSE_ID, true)

	authorizer := func(appId, authToken string, logger *gosteno.Logger) bool {
		return proxy.adminAuthorize(authToken, logger)
	}

	authorized, errorMessage := proxy.isAuthorized(authorizer, FIREHOSE_ID, authToken, clientAddress)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", errorMessage.GetMessage())
		return
	}

	proxy.serveWithDoppler(writer, request, dopplerEndpoint)

}

func (proxy *Proxy) serveAppLogs(writer http.ResponseWriter, request *http.Request) {
	clientAddress := request.RemoteAddr
	authToken := getAuthToken(request)

	validPaths := regexp.MustCompile("^/apps/(.*)/(recentlogs|stream)$")
	matches := validPaths.FindStringSubmatch(request.URL.Path)
	if len(matches) != 3 {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", request.URL.Path)
		return
	}
	appId := matches[1]

	if appId == "" {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "App ID missing. Make request to /apps/APP_ID/%s", matches[2])
		return
	}

	authorizer := func(appId, authToken string, logger *gosteno.Logger) bool {
		return proxy.logAuthorize(authToken, appId, logger)
	}

	authorized, errorMessage := proxy.isAuthorized(authorizer, appId, authToken, clientAddress)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", errorMessage.GetMessage())
		return
	}

	endpoint_type := matches[2]
	reconnect := endpoint_type != "recentlogs"

	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(endpoint_type, appId, reconnect)

	proxy.serveWithDoppler(writer, request, dopplerEndpoint)
}

func (proxy *Proxy) serveWithDoppler(writer http.ResponseWriter, request *http.Request, dopplerEndpoint doppler_endpoint.DopplerEndpoint) {
	messagesChan := make(chan []byte, 100)
	stopChan := make(chan struct{})
	defer close(stopChan)

	go proxy.connector.Connect(dopplerEndpoint, messagesChan, stopChan)

	handler := dopplerEndpoint.HProvider(messagesChan, proxy.logger)
	handler.ServeHTTP(writer, request)
}

func (proxy *Proxy) isAuthorized(authorizer Authorizer, appId, authToken string, clientAddress string) (bool, *logmessage.LogMessage) {
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

	if !authorizer(appId, authToken, proxy.logger) {
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
