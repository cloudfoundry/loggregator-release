package proxy

import (
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

var MetricsInterval = 15 * time.Second

type Health interface {
	Set(name string, value float64)
	Inc(name string)
}

type DopplerProxy struct {
	*mux.Router

	numFirehoses  int64
	numAppStreams int64

	health Health

	firehoseConnMetric  *metricemitter.Gauge
	appStreamConnMetric *metricemitter.Gauge
}

type GrpcConnector interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error)
}

type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name string, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
	EmitEvent(title string, body string)
}

func NewDopplerProxy(
	logAuthorizer auth.LogAccessAuthorizer,
	adminAuthorizer auth.AdminAccessAuthorizer,
	grpcConn GrpcConnector,
	cookieDomain string,
	timeout time.Duration,
	slowConsumerTimeout time.Duration,
	m MetricClient,
	health Health,
	recentLogsHandler http.Handler,
	disableAccessControl bool,
	logCacheClient LogCacheClient,
) *DopplerProxy {
	// metric-documentation-v2: (doppler_proxy.firehoses) Number of open firehose streams
	firehoseConnMetric := m.NewGauge("doppler_proxy.firehoses", "connections",
		metricemitter.WithVersion(2, 0),
	)

	// metric-documentation-v2: (doppler_proxy.app_streams) Number of open app streams
	appStreamConnMetric := m.NewGauge("doppler_proxy.app_streams", "connections",
		metricemitter.WithVersion(2, 0),
	)

	r := mux.NewRouter()

	adminAccessMiddleware := NewAdminAccessMiddleware(adminAuthorizer)
	logAccessMiddleware := NewLogAccessMiddleware(logAuthorizer, disableAccessControl)
	corsMiddleware := NewCORSMiddleware()

	r.Handle(
		"/set-cookie",
		corsMiddleware.Wrap(
			NewSetCookieHandler(cookieDomain),
			AllowCredentials(),
			AllowHeader("Content-Type"),
		),
	)

	r.Handle(
		"/apps/{appID}/recentlogs",
		corsMiddleware.Wrap(
			logAccessMiddleware.Wrap(recentLogsHandler),
			AllowCredentials(),
		),
	)

	wsServer := NewWebSocketServer(slowConsumerTimeout, m, health)
	streamHandler := NewStreamHandler(grpcConn, wsServer, m)
	r.Handle(
		"/apps/{appID}/stream",
		logAccessMiddleware.Wrap(streamHandler),
	)

	firehoseHandler := NewFirehoseHandler(grpcConn, wsServer, m)
	r.Handle(
		"/firehose/{subID}",
		adminAccessMiddleware.Wrap(firehoseHandler),
	)

	d := &DopplerProxy{
		Router:              r,
		health:              health,
		firehoseConnMetric:  firehoseConnMetric,
		appStreamConnMetric: appStreamConnMetric,
	}

	go d.emitMetrics(firehoseHandler, streamHandler)

	return d
}

func (d *DopplerProxy) emitMetrics(firehose *FirehoseHandler, stream *StreamHandler) {
	for range time.Tick(MetricsInterval) {
		d.firehoseConnMetric.Set(float64(firehose.Count()))
		d.health.Set("firehoseStreamCount", float64(firehose.Count()))

		d.appStreamConnMetric.Set(float64(stream.Count()))
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
