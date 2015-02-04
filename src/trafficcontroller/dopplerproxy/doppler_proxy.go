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
	"strings"
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
	cookieDomain   string
	logger         *gosteno.Logger
	cfcomponent.Component
}

type RequestTranslator func(request *http.Request) (*http.Request, error)

type Authorizer func(authToken string, appId string, logger *gosteno.Logger) (bool, error)

func NewDopplerProxy(logAuthorize authorization.LogAccessAuthorizer, adminAuthorizer authorization.AdminAccessAuthorizer, connector channel_group_connector.ChannelGroupConnector, config cfcomponent.Config, translator RequestTranslator, cookieDomain string, logger *gosteno.Logger) *Proxy {
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
		cookieDomain:   cookieDomain,
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

	endpointName := strings.Split(translatedRequest.URL.Path, "/")[1]

	switch endpointName {
	case "firehose":
		proxy.serveFirehose(writer, translatedRequest)
	case "apps":
		proxy.serveAppLogs(writer, translatedRequest)
	case "set-cookie":
		proxy.serveSetCookie(writer, translatedRequest, proxy.cookieDomain)
	default:
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found. %s", request.URL.Path)
	}
}

func (proxy *Proxy) serveFirehose(writer http.ResponseWriter, request *http.Request) {
	clientAddress := request.RemoteAddr
	authToken := getAuthToken(request)

	firehoseParams := strings.Split(request.URL.Path, "/")[2:]

	var firehoseSubscriptionId string
	if len(firehoseParams) == 1 {
		firehoseSubscriptionId = firehoseParams[0]
	}

	if len(firehoseParams) != 1 || firehoseSubscriptionId == "" {
		writer.Header().Set("WWW-Authenticate", "Basic")

		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Firehose SUBSCRIPTION_ID missing. Make request to /firehose/SUBSCRIPTION_ID")
		return
	}

	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(FIREHOSE_ID, firehoseSubscriptionId, true)

	authorizer := func(authToken string, appId string, logger *gosteno.Logger) (bool, error) {
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

	validPaths := regexp.MustCompile("^/apps/(.*)/(recentlogs|stream|containermetrics)$")
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

	authorizer := func(authToken string, appId string, logger *gosteno.Logger) (bool, error) {
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
	reconnect := endpoint_type != "recentlogs" && endpoint_type != "containermetrics"

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
	if authorized, err := authorizer(authToken, appId, proxy.logger); !authorized {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		proxy.logger.Warn(message)
		return false, newLogMessage([]byte(err.Error()))
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

func (proxy *Proxy) serveSetCookie(writer http.ResponseWriter, request *http.Request, cookieDomain string) {
	err := request.ParseForm()
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
	}

	cookieName := request.FormValue("CookieName")
	cookieValue := request.FormValue("CookieValue")
	origin := request.Header.Get("Origin")

	http.SetCookie(writer, &http.Cookie{Name: cookieName, Value: cookieValue, Domain: cookieDomain, Secure: true})

	writer.Header().Add("Access-Control-Allow-Credentials", "true")
	writer.Header().Add("Access-Control-Allow-Origin", origin)

	proxy.logger.Debugf("Proxy: Set cookie name '%s' for origin '%s' on domain '%s'", cookieName, origin, cookieDomain)
}

type TrafficControllerMonitor struct {
}

func (hm TrafficControllerMonitor) Ok() bool {
	return true
}
