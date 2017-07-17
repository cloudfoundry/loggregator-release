package clientpool

import (
	v1 "code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"golang.org/x/net/context"

	"google.golang.org/grpc/stats"
)

// StatsHandler implements the stats.Handler interface. It is passed to
// each dialer in a metron to calculate how much data is being egressed.
// It knows how to calculate both v1 and v2 envelopes. It only implements
// HandleRPC for OutPayload messages.
// It should be constructed with NewStatsHandler.
type StatsHandler struct {
	tracker Tracker
}

// Tracker is used to aggregate each envelope's size in bytes.
type Tracker interface {
	// Track is invoked each time HandleRPC is invoked with egress information
	// about an envelope.
	Track(count, size int)
}

// NewStatsHandler constructs a new StatsHandler.
func NewStatsHandler(tracker Tracker) *StatsHandler {
	return &StatsHandler{
		tracker: tracker,
	}
}

// HandleRPC only looks for OutPayload messages. It is a NOP for any other
// type. It either calculates the total bytes from v1.EnvelopeData or the
// average envelope size across a batch from v2.EnvelopeBatch.
func (s *StatsHandler) HandleRPC(ctx context.Context, stat stats.RPCStats) {
	out, ok := stat.(*stats.OutPayload)
	if !ok {
		return
	}

	switch v := out.Payload.(type) {
	case *v1.EnvelopeData:
		s.tracker.Track(1, len(v.Payload))
	case *v2.Envelope:
		s.tracker.Track(1, out.Length)
	case *v2.EnvelopeBatch:
		s.tracker.Track(len(v.Batch), out.Length)
	}
}

// TagRPC implements stats.Handler. It is a NOP.
func (s *StatsHandler) TagRPC(ctx context.Context, stat *stats.RPCTagInfo) context.Context {
	// NOP
	return ctx
}

// TagConn implements stats.Handler. It is a NOP.
func (s *StatsHandler) TagConn(ctx context.Context, stat *stats.ConnTagInfo) context.Context {
	// NOP
	return ctx
}

// HandleConn implements stats.Handler. It is a NOP.
func (s *StatsHandler) HandleConn(context.Context, stats.ConnStats) {
	// NOP
}
