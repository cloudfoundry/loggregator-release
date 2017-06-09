package proxy

import (
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"
	"plumbing"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const (
	FIREHOSE_ID     = "firehose"
	metricsInterval = time.Second
)

type Health interface {
	Set(name string, value float64)
}

type DopplerProxy struct {
	*mux.Router

	numFirehoses  int64
	numAppStreams int64

	metricSender metricSender
	health       Health
}

type grpcConnector interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error)
	ContainerMetrics(ctx context.Context, appID string) [][]byte
	RecentLogs(ctx context.Context, appID string) [][]byte
}

type metricSender interface {
	SendValue(name string, value float64, unit string) error
}

func NewDopplerProxy(
	logAuthorizer auth.LogAccessAuthorizer,
	adminAuthorizer auth.AdminAccessAuthorizer,
	grpcConn grpcConnector,
	cookieDomain string,
	timeout time.Duration,
	m metricSender,
	health Health,
) *DopplerProxy {
	r := mux.NewRouter()

	adminAccessMiddleware := NewAdminAccessMiddleware(adminAuthorizer)
	logAccessMiddleware := NewLogAccessMiddleware(logAuthorizer)

	r.Handle("/set-cookie", NewSetCookieHandler(cookieDomain))

	containerMetricsHandler := NewContainerMetricsHandler(grpcConn, timeout, m)
	r.Handle("/apps/{appID}/containermetrics", logAccessMiddleware.Wrap(
		containerMetricsHandler,
	))

	recentLogsHandler := NewRecentLogsHandler(grpcConn, timeout, m)
	r.Handle("/apps/{appID}/recentlogs", logAccessMiddleware.Wrap(recentLogsHandler))

	wsServer := &WebSocketServer{
		MetricSender: m,
	}
	streamHandler := NewStreamHandler(grpcConn, wsServer)
	r.Handle("/apps/{appID}/stream", logAccessMiddleware.Wrap(streamHandler))

	firehoseHandler := NewFirehoseHandler(grpcConn, wsServer)
	r.Handle("/firehose/{subID}", adminAccessMiddleware.Wrap(firehoseHandler))

	d := &DopplerProxy{
		Router:       r,
		metricSender: m,
		health:       health,
	}

	go d.emitMetrics(firehoseHandler, streamHandler)

	return d
}

func (d *DopplerProxy) emitMetrics(firehose *FirehoseHandler, stream *StreamHandler) {
	for range time.Tick(metricsInterval) {
		// metric-documentation-v1: (dopplerProxy.firehoses) Number of open firehose streams
		d.metricSender.SendValue("dopplerProxy.firehoses", float64(firehose.Count()), "connections")
		d.health.Set("firehoseStreamCount", float64(firehose.Count()))

		// metric-documentation-v1: (dopplerProxy.appStreams) Number of open app streams
		d.metricSender.SendValue("dopplerProxy.appStreams", float64(stream.Count()), "connections")
		d.health.Set("appStreamCount", float64(stream.Count()))
	}
}

func serveMultiPartResponse(rw http.ResponseWriter, messages [][]byte) {
	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for _, message := range messages {
		partWriter, err := mp.CreatePart(nil)
		if err != nil {
			log.Printf("http handler: Client went away while serving recent logs")
			return
		}

		partWriter.Write(message)
	}
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
