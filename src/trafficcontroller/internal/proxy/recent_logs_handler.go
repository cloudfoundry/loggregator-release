package proxy

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	logcache "code.cloudfoundry.org/log-cache/pkg/client"
	"code.cloudfoundry.org/log-cache/pkg/rpc/logcache_v1"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
)

const backoffRate = 0.75

type LogCacheClient interface {
	Read(
		ctx context.Context,
		sourceID string,
		start time.Time,
		opts ...logcache.ReadOption,
	) ([]*loggregator_v2.Envelope, error)
}

type RecentLogsHandler struct {
	recentLogProvider   LogCacheClient
	timeout             time.Duration
	latencyMetric       *metricemitter.Gauge
	logCacheFailsMetric *metricemitter.Counter
	logCacheEnabled     bool
}

func NewRecentLogsHandler(
	recentLogProvider LogCacheClient,
	t time.Duration,
	m MetricClient,
	logCacheEnabled bool,
) *RecentLogsHandler {
	// metric-documentation-v2: (doppler_proxy.recent_logs_latency) Measures
	// amount of time to serve the request for recent logs
	latencyMetric := m.NewGauge("doppler_proxy.recent_logs_latency", "ms",
		metricemitter.WithVersion(2, 0),
	)

	logCacheFailsMetric := m.NewCounter("doppler_proxy.log_cache_failure",
		metricemitter.WithVersion(2, 0),
	)

	return &RecentLogsHandler{
		recentLogProvider:   recentLogProvider,
		timeout:             t,
		latencyMetric:       latencyMetric,
		logCacheFailsMetric: logCacheFailsMetric,
		logCacheEnabled:     logCacheEnabled,
	}
}

func (h *RecentLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.logCacheEnabled {
		envelopeBytes, err := (&events.Envelope{
			Origin:    proto.String("loggregator.trafficcontroller"),
			EventType: events.Envelope_LogMessage.Enum(),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			LogMessage: &events.LogMessage{
				Message:     []byte("recent log endpoint requires a log cache. please talk to you operator"),
				Timestamp:   proto.Int64(time.Now().UnixNano()),
				MessageType: events.LogMessage_ERR.Enum(),
				SourceType:  proto.String("Loggregator"),
			},
		}).Marshal()
		if err != nil {
			log.Panicf("A safe envelope marshalling failed: %s", err)
		}
		resp := [][]byte{envelopeBytes}
		serveMultiPartResponse(w, resp)
		return
	}

	startTime := time.Now()
	defer func() {
		elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
		h.latencyMetric.Set(elapsedMillisecond)
	}()

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = context.WithDeadline(ctx, time.Now().Add(h.timeout))
	defer cancel()

	limit, ok := limitFrom(r)
	if !ok {
		limit = 1000
	}

	envelopes, err := backoffSearchForLogs(limit, ctx, appID, h.recentLogProvider)

	if err != nil {
		h.logCacheFailsMetric.Increment(1)
		log.Printf("error communicating with log cache: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	var resp [][]byte
	for _, v2e := range envelopes {
		// We only care about Log envelopes for recent logs.
		if _, ok := v2e.GetMessage().(*loggregator_v2.Envelope_Log); !ok {
			continue
		}

		for _, v1e := range conversion.ToV1(v2e) {
			v1bytes, err := proto.Marshal(v1e)
			if err != nil {
				log.Printf("error marshalling v1 envelope for recent log response: %s", err)
				continue
			}
			resp = append(resp, v1bytes)
		}
	}
	serveMultiPartResponse(w, resp)
}

func backoffSearchForLogs(limit int, ctx context.Context, appID string, logProvider LogCacheClient) ([]*loggregator_v2.Envelope, error) {
	envelopes, err := logProvider.Read(
		ctx,
		appID,
		time.Unix(0, 0),
		logcache.WithLimit(limit),
		logcache.WithDescending(),
		logcache.WithEnvelopeTypes(logcache_v1.EnvelopeType_LOG),
	)

	if err != nil {
		if strings.Contains(err.Error(), "received message larger than max") {
			return backoffSearchForLogs(int(float64(limit)*backoffRate), ctx, appID, logProvider)
		}
		return nil, err
	}

	return envelopes, nil
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
