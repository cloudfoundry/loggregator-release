package proxy

import (
	"context"
	"net/http"
	"sync/atomic"

	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/gorilla/mux"
)

type StreamHandler struct {
	server   *WebSocketServer
	grpcConn grpcConnector
	counter  int64
	sender   MetricSender
}

func NewStreamHandler(grpcConn grpcConnector, w *WebSocketServer, m MetricSender) *StreamHandler {
	return &StreamHandler{
		grpcConn: grpcConn,
		server:   w,
		sender:   m,
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

	h.server.serveWS(w, r, client, h.sender.IncrementEgressStream)
}

func (h *StreamHandler) Count() int64 {
	return atomic.LoadInt64(&h.counter)
}
