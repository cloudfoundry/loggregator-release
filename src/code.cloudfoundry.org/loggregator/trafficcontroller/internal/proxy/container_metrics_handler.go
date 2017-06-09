package proxy

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
)

type ContainerMetricsHandler struct {
	grpcConn     grpcConnector
	timeout      time.Duration
	metricSender metricSender
}

func NewContainerMetricsHandler(
	grpcConn grpcConnector,
	t time.Duration,
	m metricSender,
) *ContainerMetricsHandler {
	return &ContainerMetricsHandler{
		grpcConn:     grpcConn,
		timeout:      t,
		metricSender: m,
	}
}

func (h *ContainerMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
		// metric-documentation-v1: (dopplerProxy.containermetricsLatency)
		// Measures amount of time to serve the request for container metrics
		h.metricSender.SendValue("dopplerProxy.containermetricsLatency", elapsedMillisecond, "ms")
	}()

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
