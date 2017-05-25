package proxy

import (
	"log"
	"net/http"
	"time"
)

type WebSocketServer struct {
	metricSender metricSender
}

func (s *WebSocketServer) serveWS(
	endpointType string,
	streamID string,
	w http.ResponseWriter,
	r *http.Request,
	recv func() ([]byte, error),
) {
	dopplerEndpoint := NewDopplerEndpoint(endpointType, streamID, false)
	data := make(chan []byte)
	handler := dopplerEndpoint.HProvider(data)

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
