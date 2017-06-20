package proxy

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type RecentLogsHandler struct {
	grpcConn     grpcConnector
	timeout      time.Duration
	metricSender metricSender
}

func NewRecentLogsHandler(
	grpcConn grpcConnector,
	t time.Duration,
	m metricSender,
) *RecentLogsHandler {
	return &RecentLogsHandler{
		grpcConn:     grpcConn,
		timeout:      t,
		metricSender: m,
	}
}

func (h *RecentLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
		// metric-documentation-v1: (dopplerProxy.recentlogsLatency) Measures
		// amount of time to serve the request for recent logs
		h.metricSender.SendValue("dopplerProxy.recentlogsLatency", elapsedMillisecond, "ms")
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
