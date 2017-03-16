package v2

import (
	"log"
	"metric"
	v2 "plumbing/v2"
	"time"
)

type DataSetter interface {
	Set(e *v2.Envelope)
}

type Receiver struct {
	dataSetter DataSetter
}

func NewReceiver(dataSetter DataSetter) *Receiver {
	return &Receiver{
		dataSetter: dataSetter,
	}
}

func (s *Receiver) Sender(sender v2.Ingress_SenderServer) error {
	var count uint64
	lastEmitted := time.Now()
	for {
		e, err := sender.Recv()
		if err != nil {
			log.Printf("Failed to receive data: %s", err)
			return err
		}

		s.dataSetter.Set(e)

		count++
		if count >= 1000 || time.Since(lastEmitted) > 5*time.Second {
			// metric:v2 (loggregator.metron.ingress) The number of received
			// messages over Metrons V2 gRPC API.
			metric.IncCounter("ingress",
				metric.WithIncrement(count),
				metric.WithVersion(2, 0),
			)
			lastEmitted = time.Now()
			log.Printf("Ingressed (v2) %d envelopes", count)
			count = 0
		}
	}

	return nil
}
