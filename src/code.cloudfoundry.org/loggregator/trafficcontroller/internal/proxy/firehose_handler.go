package proxy

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"

	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/gorilla/mux"
)

type FirehoseHandler struct {
	server   *WebSocketServer
	grpcConn grpcConnector
	counter  int64
}

func NewFirehoseHandler(grpcConn grpcConnector, w *WebSocketServer) *FirehoseHandler {
	return &FirehoseHandler{
		grpcConn: grpcConn,
		server:   w,
	}
}

func (h *FirehoseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.counter, 1)
	defer atomic.AddInt64(&h.counter, -1)

	subID := mux.Vars(r)["subID"]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var filter *plumbing.Filter
	switch r.URL.Query().Get("filter-type") {
	case "logs":
		filter = &plumbing.Filter{
			Message: &plumbing.Filter_Log{
				Log: &plumbing.LogFilter{},
			},
		}
	case "metrics":
		filter = &plumbing.Filter{
			Message: &plumbing.Filter_Metric{
				Metric: &plumbing.MetricFilter{},
			},
		}
	default:
		filter = nil
	}

	client, err := h.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
		ShardID: subID,
		Filter:  filter,
	})
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("error occurred when subscribing to doppler: %s", err)
		return
	}

	h.server.serveWS(w, r, client)
}

func (h *FirehoseHandler) Count() int64 {
	return atomic.LoadInt64(&h.counter)
}
