package proxy

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"

	"github.com/gorilla/mux"
)

type RecentLogsHandler struct {
	grpcConn      grpcConnector
	timeout       time.Duration
	latencyMetric *metricemitter.Gauge
}

func NewRecentLogsHandler(
	grpcConn grpcConnector,
	t time.Duration,
	m MetricClient,
) *RecentLogsHandler {
	// metric-documentation-v2: (doppler_proxy.recent_logs_latency) Measures
	// amount of time to serve the request for recent logs
	latencyMetric := m.NewGauge("doppler_proxy.recent_logs_latency", "ms",
		metricemitter.WithVersion(2, 0),
	)

	return &RecentLogsHandler{
		grpcConn:      grpcConn,
		timeout:       t,
		latencyMetric: latencyMetric,
	}
}

func (h *RecentLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
		h.latencyMetric.Set(elapsedMillisecond)
	}()

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = context.WithDeadline(ctx, time.Now().Add(h.timeout))
	defer cancel()

	resp := h.grpcConn.RecentLogs(ctx, appID)
	limit, ok := limitFrom(r)
	if ok && len(resp) > limit {
		resp = resp[:limit]
	}

	serveMultiPartResponse(w, resp)
}

func limitFrom(r *http.Request) (int, bool) {
	query := r.URL.Query()
	values, ok := query["limit"]
	if !ok {
		return 0, false
	}

	value, err := strconv.Atoi(values[0])
	if err != nil || value < 0 {
		return 0, false
	}

	return value, true
}
