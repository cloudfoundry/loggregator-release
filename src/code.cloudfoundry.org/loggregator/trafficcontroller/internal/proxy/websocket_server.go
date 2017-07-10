package proxy

import (
	"log"
	"net/http"
	"time"
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
	w http.ResponseWriter,
	r *http.Request,
	recv func() ([]byte, error),
	incEgressMetric func(),
) {
	data := make(chan []byte)

	handler := NewWebsocketHandler(
		data,
		websocketKeepAliveDuration,
		s.metricSender,
		incEgressMetric,
	)

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
				// metric-documentation-v1: (dopplerProxy.slowConsumer) A slow
				// consumer of the websocket stream
				s.metricSender.SendValue("dopplerProxy.slowConsumer", 1, "consumer")
				log.Print("Doppler Proxy: Slow Consumer")
				return
			}
		}
	}()

	handler.ServeHTTP(w, r)
}
