package dopplerproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"plumbing"
	"trafficcontroller/authorization"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const FIREHOSE_ID = "firehose"

type Proxy struct {
	mux.Router

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
	p := &Proxy{
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		connector:      connector,
		grpcConn:       grpcConn,
		cookieDomain:   cookieDomain,
		logger:         logger,
	}
	r := mux.NewRouter()
	p.Router = *r
	p.HandleFunc("/apps/{appID}/stream", p.stream)
	p.HandleFunc("/apps/{appID}/recentlogs", p.recentlogs)
	p.HandleFunc("/apps/{appID}/containermetrics", p.containermetrics)
	p.HandleFunc("/firehose/{subID}", p.firehose)
	p.HandleFunc("/set-cookie", p.setcookie)
	return p
}

func (p *Proxy) firehose(w http.ResponseWriter, r *http.Request) {
	subID := mux.Vars(r)["subID"]
	p.serveFirehose(subID, w, r)
}

func (p *Proxy) stream(w http.ResponseWriter, r *http.Request) {
	p.serveAppLogs("stream", mux.Vars(r)["appID"], w, r)
}

func (p *Proxy) recentlogs(w http.ResponseWriter, r *http.Request) {
	p.serveAppLogs("recentlogs", mux.Vars(r)["appID"], w, r)
	sendMetric("recentlogs", time.Now())
}

func (p *Proxy) containermetrics(w http.ResponseWriter, r *http.Request) {
	p.serveAppLogs("containermetrics", mux.Vars(r)["appID"], w, r)
	sendMetric("containermetrics", time.Now())
}

func (p *Proxy) setcookie(w http.ResponseWriter, r *http.Request) {
	p.serveSetCookie(w, r, p.cookieDomain)
}

func (p *Proxy) serveFirehose(firehoseSubscriptionId string, writer http.ResponseWriter, request *http.Request) {
	authToken := getAuthToken(request)

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
func (p *Proxy) serveAppLogs(requestPath, appID string, writer http.ResponseWriter, request *http.Request) {
	authToken := getAuthToken(request)

	status, err := p.logAuthorize(authToken, appID, p.logger)
	if status != http.StatusOK {
		p.logger.Warndf(map[string]interface{}{
			"status": status,
			"err":    err,
		}, "auth token not authorized to access appID [%s].", appID)
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

	if requestPath == "recentlogs" || requestPath == "containermetrics" {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(requestPath, appID, false)
		p.serveWithDoppler(writer, request, dopplerEndpoint)
		return
	}

	client, err := p.grpcConn.Stream(ctx, &plumbing.StreamRequest{
		AppID: appID,
	})

	if err != nil {
		p.logger.Errorf("Failed to stream from doppler: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.serveWS(requestPath, appID, writer, request, client.Recv)
}

func (p *Proxy) serveWS(endpointType, streamID string, w http.ResponseWriter, r *http.Request, recv func() (*plumbing.Response, error)) {
	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(endpointType, streamID, false)
	data := make(chan []byte)
	handler := dopplerEndpoint.HProvider(data, p.logger)

	go func() {
		defer close(data)
		for {
			resp, err := recv()
			if err != nil {
				p.logger.Error(err.Error())
				return
			}

			if resp == nil {
				continue
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
