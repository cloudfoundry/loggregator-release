package proxy

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
)

type ContainerMetricsHandler struct {
	grpcConn      grpcConnector
	timeout       time.Duration
	latencyMetric *metricemitter.Gauge
}

func NewContainerMetricsHandler(
	grpcConn grpcConnector,
	t time.Duration,
	m MetricClient,
) *ContainerMetricsHandler {
	// metric-documentation-v2: (doppler_proxy.container_metrics_latency)
	// Measures amount of time to serve the request for container metrics
	latencyMetric := m.NewGauge("doppler_proxy.container_metrics_latency", "ms",
		metricemitter.WithVersion(2, 0),
	)

	return &ContainerMetricsHandler{
		grpcConn:      grpcConn,
		timeout:       t,
		latencyMetric: latencyMetric,
	}
}

func (h *ContainerMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)

		h.latencyMetric.Set(elapsedMillisecond)
	}()

	appID := mux.Vars(r)["appID"]

	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = context.WithDeadline(ctx, time.Now().Add(h.timeout))
	defer cancel()

	resp := deDupe(h.grpcConn.ContainerMetrics(ctx, appID))

	serveMultiPartResponse(w, resp)
}

func deDupe(input [][]byte) [][]byte {
	messages := make(map[int32]*events.Envelope)

	for _, message := range input {
		var envelope events.Envelope
		proto.Unmarshal(message, &envelope)
		cm := envelope.GetContainerMetric()

		oldEnvelope, ok := messages[cm.GetInstanceIndex()]
		if !ok || oldEnvelope.GetTimestamp() < envelope.GetTimestamp() {
			messages[cm.GetInstanceIndex()] = &envelope
		}
	}

	output := make([][]byte, 0, len(messages))

	for _, envelope := range messages {
		bytes, _ := proto.Marshal(envelope)
		output = append(output, bytes)
	}
	return output
}
