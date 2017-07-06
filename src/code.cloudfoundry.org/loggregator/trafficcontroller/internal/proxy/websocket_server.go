package proxy

import (
	"log"
	"net/http"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const websocketKeepAliveDuration = 30 * time.Second

type WebSocketServer struct {
	metricSender MetricSender
}

func NewWebSocketServer(m MetricSender) *WebSocketServer {
	return &WebSocketServer{
		metricSender: m,
	}
}

func (s *WebSocketServer) serveWS(
	endpointType string,
	w http.ResponseWriter,
	r *http.Request,
	recv func() ([]byte, error),
) {
	data := make(chan []byte)

	var handler http.Handler
	switch endpointType {
	case "recentlogs":
		handler = NewHttpHandler(data)
	case "containermetrics":
		handler = NewHttpHandler(DeDupe(data))
	default:
		handler = NewWebsocketHandler(data, websocketKeepAliveDuration)
	}

	go func() {
		defer close(data)
		timer := time.NewTimer(5 * time.Second)
		timer.Stop()
		for {
			resp, err := recv()
			if err != nil {
				log.Printf("error receiving from doppler via gRPC %s", err)
				return
			}

			if resp == nil {
				continue
			}

			timer.Reset(5 * time.Second)
			select {
			case data <- resp:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				// metric-documentation-v1: (dopplerProxy.slowConsumer) A slow consumer of the
				// websocket stream
				s.metricSender.SendValue("dopplerProxy.slowConsumer", 1, "consumer")
				log.Print("Doppler Proxy: Slow Consumer")
				return
			}
		}
	}()

	handler.ServeHTTP(w, r)
}

func DeDupe(input <-chan []byte) <-chan []byte {
	messages := make(map[int32]*events.Envelope)
	for message := range input {
		var envelope events.Envelope
		proto.Unmarshal(message, &envelope)
		cm := envelope.GetContainerMetric()

		oldEnvelope, ok := messages[cm.GetInstanceIndex()]
		if !ok || oldEnvelope.GetTimestamp() < envelope.GetTimestamp() {
			messages[cm.GetInstanceIndex()] = &envelope
		}
	}

	output := make(chan []byte, len(messages))

	for _, envelope := range messages {
		bytes, _ := proto.Marshal(envelope)
		output <- bytes
	}
	close(output)
	return output
}
