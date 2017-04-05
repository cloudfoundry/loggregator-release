package proxy

import (
	"context"
	"net/http"
	"plumbing"
	"sync/atomic"

	"github.com/gorilla/mux"
)

type StreamHandler struct {
	grpcConn grpcConnector
	counter  int64
}

func NewStreamHandler(grpcConn grpcConnector) *StreamHandler {
	return &StreamHandler{grpcConn: grpcConn}
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

	serveWS("stream", appID, w, r, client)
}

func (h *StreamHandler) Count() int64 {
	return atomic.LoadInt64(&h.counter)
}
