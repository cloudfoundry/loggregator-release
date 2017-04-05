package proxy

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type RecentLogsHandler struct {
	grpcConn grpcConnector
	timeout  time.Duration
}

func NewRecentLogsHandler(grpcConn grpcConnector, t time.Duration) *RecentLogsHandler {
	return &RecentLogsHandler{
		grpcConn: grpcConn,
		timeout:  t,
	}
}

func (h *RecentLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer sendLatencyMetric("recentlogs", startTime)

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = context.WithDeadline(ctx, time.Now().Add(h.timeout))
	defer cancel()

	resp := h.grpcConn.RecentLogs(ctx, appID)
	if err := ctx.Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("recentlogs request encountered an error: %s", err)
		return
	}

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
