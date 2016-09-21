package dopplerproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"plumbing"
	"strings"
	"trafficcontroller/authorization"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"golang.org/x/net/context"
)

const FIREHOSE_ID = "firehose"

type Proxy struct {
	logAuthorize   authorization.LogAccessAuthorizer
	adminAuthorize authorization.AdminAccessAuthorizer
	connector      channelGroupConnector
	grpcConn       grpcConnector
	cookieDomain   string
	logger         *gosteno.Logger
}

type channelGroupConnector interface {
	Connect(dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{})
}

type grpcConnector interface {
	Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (grpcconnector.Receiver, error)
	Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (grpcconnector.Receiver, error)
}

func NewDopplerProxy(
	logAuthorize authorization.LogAccessAuthorizer,
	adminAuthorizer authorization.AdminAccessAuthorizer,
	connector channelGroupConnector,
	grpcConn grpcConnector,
	cookieDomain string,
	logger *gosteno.Logger,
) *Proxy {
	return &Proxy{
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		connector:      connector,
		grpcConn:       grpcConn,
		cookieDomain:   cookieDomain,
		logger:         logger,
	}
}

func (p *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestStartTime := time.Now()
	p.logger.Debugf("doppler proxy: ServeHTTP entered with request %v", request.URL)
	defer p.logger.Debugf("doppler proxy: ServeHTTP exited")

	if request.Method == "HEAD" {
		return
	}

	paths := strings.Split(request.URL.Path, "/")

	endpointName := paths[1]
	switch endpointName {
	case "firehose":
		p.serveFirehose(paths, writer, request)
	case "apps":
		p.serveAppLogs(paths, writer, request)
		if len(paths) > 3 {
			endpoint := paths[3]
			if endpoint != "" && (endpoint == "recentlogs" || endpoint == "containermetrics") {
				sendMetric(endpoint, requestStartTime)
			}
		}
	case "set-cookie":
		p.serveSetCookie(writer, request, p.cookieDomain)
	default:
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprint(writer, "Resource Not Found.")
	}
}

func (p *Proxy) serveFirehose(paths []string, writer http.ResponseWriter, request *http.Request) {
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

	authorized, err := p.adminAuthorize(authToken, p.logger)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", err.Error())
		p.logger.Warnf("auth token not authorized to access firehose")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := p.grpcConn.Firehose(ctx, &plumbing.FirehoseRequest{
		SubID: firehoseSubscriptionId,
	})

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.serveWS(FIREHOSE_ID, firehoseSubscriptionId, writer, request, client.Recv)
}

// "^/apps/(.*)/(recentlogs|stream|containermetrics)$"
func (p *Proxy) serveAppLogs(paths []string, writer http.ResponseWriter, request *http.Request) {
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

	status, err := p.logAuthorize(authToken, appId, p.logger)
	if status != http.StatusOK {
		p.logger.Warndf(map[string]interface{}{
			"status": status,
			"err":    err,
		}, "auth token not authorized to access appId [%s].", appId)
		switch status {
		case http.StatusUnauthorized:
			writer.WriteHeader(status)
			writer.Header().Set("WWW-Authenticate", "Basic")
		case http.StatusForbidden, http.StatusNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}

		writer.WriteHeader(status)
		writer.Write([]byte(http.StatusText(status)))

		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	endpointType := paths[3]
	if endpointType == "recentlogs" || endpointType == "containermetrics" {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(endpointType, appId, false)
		p.serveWithDoppler(writer, request, dopplerEndpoint)
		return
	}

	client, err := p.grpcConn.Stream(ctx, &plumbing.StreamRequest{
		AppID: appId,
	})

	if err != nil {
		p.logger.Errorf("Failed to stream from doppler: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.serveWS(endpointType, appId, writer, request, client.Recv)
}

func (p *Proxy) serveWS(endpointType, streamID string, w http.ResponseWriter, r *http.Request, recv func() (*plumbing.Response, error)) {
	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(endpointType, streamID, false)
	data := make(chan []byte)
	handler := dopplerEndpoint.HProvider(data, p.logger)

	go func() {
		defer close(data)
		for {
			resp, err := recv()
			if resp == nil {
				continue
			}

			if err != nil {
				p.logger.Error(err.Error())
				return
			}

			data <- resp.Payload
		}
	}()
	handler.ServeHTTP(w, r)
}

func (p *Proxy) serveWithDoppler(writer http.ResponseWriter, request *http.Request, dopplerEndpoint doppler_endpoint.DopplerEndpoint) {
	messagesChan := make(chan []byte, 100)
	stopChan := make(chan struct{})
	defer close(stopChan)

	go p.connector.Connect(dopplerEndpoint, messagesChan, stopChan)

	handler := dopplerEndpoint.HProvider(messagesChan, p.logger)
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

func (p *Proxy) serveSetCookie(writer http.ResponseWriter, request *http.Request, cookieDomain string) {
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

	p.logger.Debugf("Proxy: Set cookie name '%s' for origin '%s' on domain '%s'", cookieName, origin, cookieDomain)
}
