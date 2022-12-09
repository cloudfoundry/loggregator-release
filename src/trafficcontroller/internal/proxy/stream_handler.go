package proxy

import (
	"context"
	"net/http"
	"sync/atomic"

	"code.cloudfoundry.org/loggregator-release/metricemitter"
	"code.cloudfoundry.org/loggregator-release/plumbing"

	"github.com/gorilla/mux"
)

type StreamHandler struct {
	server       *WebSocketServer
	grpcConn     GrpcConnector
	counter      int64
	egressMetric *metricemitter.Counter
}

func NewStreamHandler(grpcConn GrpcConnector, w *WebSocketServer, m MetricClient) *StreamHandler {
	// metric-documentation-v2: (egress) Number of envelopes egressed via
	// an app stream.
	egressMetric := m.NewCounter("egress",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(
			map[string]string{"endpoint": "stream"},
		),
	)

	return &StreamHandler{
		grpcConn:     grpcConn,
		server:       w,
		egressMetric: egressMetric,
	}
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.counter, 1)
	defer atomic.AddInt64(&h.counter, -1)

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := h.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
		Filter: &plumbing.Filter{
			AppID: appID,
		},
	})
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	h.server.ServeWS(w, r, client, h.egressMetric)
}

func (h *StreamHandler) Count() int64 {
	return atomic.LoadInt64(&h.counter)
}
