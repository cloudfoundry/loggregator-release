package web

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

// Receiver defines a function for receiving batch envelopes
type Receiver func() (*loggregator_v2.EnvelopeBatch, error)

// LogsProvder defines the interface for opening streams to the
// logs provider
type LogsProvider interface {
	Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) Receiver
}

// Handler defines a struct for servering http endpoints
type Handler struct {
	mux *http.ServeMux
}

// NewHandler returns a configured HTTP Handler
func NewHandler(lp LogsProvider, streamTimeout time.Duration) *Handler {
	mux := http.NewServeMux()

	mux.Handle("/v2/read", ReadHandler(lp, 15*time.Second, streamTimeout))

	return &Handler{mux: mux}
}

// ServeHTTP satisfies the http.ServeHTTP interface and forwards
// requests to the Handler's mux
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
