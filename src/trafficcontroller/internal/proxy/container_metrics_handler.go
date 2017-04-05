package proxy

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type ContainerMetricsHandler struct {
	grpcConn grpcConnector
	timeout  time.Duration
}

func NewContainerMetricsHandler(grpcConn grpcConnector, t time.Duration) *ContainerMetricsHandler {
	return &ContainerMetricsHandler{
		grpcConn: grpcConn,
		timeout:  t,
	}
}

func (h *ContainerMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer sendLatencyMetric("containermetrics", startTime)

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = context.WithDeadline(ctx, time.Now().Add(h.timeout))
	defer cancel()

	resp := deDupe(h.grpcConn.ContainerMetrics(ctx, appID))
	if err := ctx.Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("containermetrics request encountered an error: %s", err)
		return
	}
	serveMultiPartResponse(w, resp)
	return
}
