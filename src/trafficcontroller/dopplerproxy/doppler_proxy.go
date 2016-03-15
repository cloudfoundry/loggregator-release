package dopplerproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"trafficcontroller/authorization"
	"trafficcontroller/doppler_endpoint"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

const FIREHOSE_ID = "firehose"

type Proxy struct {
	logAuthorize   authorization.LogAccessAuthorizer
	adminAuthorize authorization.AdminAccessAuthorizer
	connector      channelGroupConnector
	translate      RequestTranslator
	cookieDomain   string
	logger         *gosteno.Logger
}

type RequestTranslator func(request *http.Request) (*http.Request, error)

type channelGroupConnector interface {
	Connect(dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{})
}

func NewDopplerProxy(logAuthorize authorization.LogAccessAuthorizer, adminAuthorizer authorization.AdminAccessAuthorizer, connector channelGroupConnector, translator RequestTranslator, cookieDomain string, logger *gosteno.Logger) *Proxy {
	return &Proxy{
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		connector:      connector,
		translate:      translator,
		cookieDomain:   cookieDomain,
		logger:         logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestStartTime := time.Now()
	proxy.logger.Debugf("doppler proxy: ServeHTTP entered with request %v", request.URL)
	defer proxy.logger.Debugf("doppler proxy: ServeHTTP exited")

	translatedRequest, err := proxy.translate(request)
	if err != nil {
		proxy.logger.Errorf("DopplerProxy.ServeHTTP: unable to translate request: %s", err.Error())
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, "Resource Not Found.")
		return
	}

	if request.Method == "HEAD" {
		return
	}

	paths := strings.Split(translatedRequest.URL.Path, "/")

	endpointName := paths[1]
	switch endpointName {
	case "firehose":
		proxy.serveFirehose(paths, writer, translatedRequest)
	case "apps":
		proxy.serveAppLogs(paths, writer, translatedRequest)
		if len(paths) > 3 {
			endpoint := paths[3]
			if endpoint != "" && (endpoint == "recentlogs" || endpoint == "containermetrics") {
				sendMetric(endpoint, requestStartTime)
			}
		}
	case "set-cookie":
		proxy.serveSetCookie(writer, translatedRequest, proxy.cookieDomain)
	default:
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, "Resource Not Found.")
	}
}

func (proxy *Proxy) serveFirehose(paths []string, writer http.ResponseWriter, request *http.Request) {
	authToken := getAuthToken(request)

	firehoseParams := paths[2:]

	var firehoseSubscriptionId string
	if len(firehoseParams) == 1 {
		firehoseSubscriptionId = firehoseParams[0]
	}

	if len(firehoseParams) != 1 || firehoseSubscriptionId == "" {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Firehose SUBSCRIPTION_ID missing. Make request to /firehose/SUBSCRIPTION_ID")
		return
	}

	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(FIREHOSE_ID, firehoseSubscriptionId, true)

	authorized, err := proxy.adminAuthorize(authToken, proxy.logger)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", err.Error())
		proxy.logger.Warnf("auth token not authorized to access firehose")
		return
	}

	proxy.serveWithDoppler(writer, request, dopplerEndpoint)
}

// "^/apps/(.*)/(recentlogs|stream|containermetrics)$"
func (proxy *Proxy) serveAppLogs(paths []string, writer http.ResponseWriter, request *http.Request) {
	badRequest := len(paths) != 4
	if !badRequest {
		switch paths[3] {
		case "recentlogs", "stream", "containermetrics":
		default:
			badRequest = true
		}
	}

	if badRequest {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "Resource Not Found.")
		return
	}

	authToken := getAuthToken(request)

	appId := paths[2]
	if appId == "" {
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(writer, "App ID missing. Make request to /apps/APP_ID/%s", paths[3])
		return
	}

	authorized, err := proxy.logAuthorize(authToken, appId, proxy.logger)

	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", err.Error())
		proxy.logger.Warnf("auth token not authorized to access appId [%s].", appId)
		return
	}

	endpoint_type := paths[3]
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

func sendMetric(metricName string, startTime time.Time) {
	elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
	metrics.SendValue(fmt.Sprintf("dopplerProxy.%sLatency", metricName), elapsedMillisecond, "ms")
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
