package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// ReadHandler returns a http.Handler that will serve logs over server sent
// events. Logs are streamed from the logs provider and written to the client
// connection. The format of the envelopes is as follows:
//
//	data: <JSON ENVELOPE BATCH>
//
//	data: <JSON ENVELOPE BATCH>
func ReadHandler(
	lp LogsProvider,
	heartbeat time.Duration,
	streamTimeout time.Duration,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errMethodNotAllowed.Write(w)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		query := r.URL.Query()

		s, err := BuildSelector(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			errStreamingUnsupported.Write(w)
			return
		}

		recv, err := lp.Stream(
			ctx,
			&loggregator_v2.EgressBatchRequest{
				ShardId:           query.Get("shard_id"),
				DeterministicName: query.Get("deterministic_name"),
				UsePreferredTags:  true,
				Selectors:         s,
			},
		)
		if err != nil {
			errStreamingUnavailable.Write(w)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher.Flush()

		data := make(chan *loggregator_v2.EnvelopeBatch)
		errs := make(chan error, 1)

		go func() {
			for {
				batch, err := recv()
				if err != nil {
					errs <- err
					return
				}

				if batch == nil {
					continue
				}

				select {
				case data <- batch:
				case <-ctx.Done():
					return
				}
			}
		}()

		heartbeatTimer := time.NewTimer(heartbeat)
		streamTimeoutTimer := time.NewTimer(streamTimeout)

		// TODO:
		//   - error events
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errs:
				status, ok := status.FromError(err)
				if ok && status.Code() != codes.Canceled {
					log.Printf("error getting logs from provider: %s", err)
				}

				return
			case batch := <-data:
				d, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(batch)
				if err != nil {
					log.Printf("error marshaling envelope batch to string: %s", err)
					return
				}

				fmt.Fprintf(w, "data: %s\n\n", string(d))
				flusher.Flush()

				if !heartbeatTimer.Stop() {
					<-heartbeatTimer.C
				}
				heartbeatTimer.Reset(heartbeat)
			case <-streamTimeoutTimer.C:
				fmt.Fprint(w, "event: closing\ndata: closing due to stream timeout\n\n")
				flusher.Flush()
				return
			case t := <-heartbeatTimer.C:
				fmt.Fprintf(w, "event: heartbeat\ndata: %d\n\n", t.Unix())
				flusher.Flush()
				heartbeatTimer.Reset(heartbeat)
			}
		}
	}
}
