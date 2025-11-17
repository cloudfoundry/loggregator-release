package proxy

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"code.cloudfoundry.org/loggregator-release/src/trafficcontroller/internal/auth"

	"github.com/gorilla/mux"
)

var MetricsInterval = 15 * time.Second

type DopplerProxy struct {
	*mux.Router

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
	slowConsumerTimeout time.Duration,
	m MetricClient,
	disableAccessControl bool,
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

	wsServer := NewWebSocketServer(slowConsumerTimeout, m)
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
		firehoseConnMetric:  firehoseConnMetric,
		appStreamConnMetric: appStreamConnMetric,
	}

	go d.emitMetrics(firehoseHandler, streamHandler)

	return d
}

func (d *DopplerProxy) emitMetrics(firehose *FirehoseHandler, stream *StreamHandler) {
	for range time.Tick(MetricsInterval) {
		d.firehoseConnMetric.Set(float64(firehose.Count()))
		d.appStreamConnMetric.Set(float64(stream.Count()))
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
