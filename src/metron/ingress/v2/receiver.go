package v2

import (
	"log"
	"metric"
	v2 "plumbing/v2"
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
	var count int
	for {
		e, err := sender.Recv()
		if err != nil {
			log.Printf("Failed to receive data: %s", err)
			return err
		}

		s.dataSetter.Set(e)

		count++
		if count%1000 == 0 {
			metric.IncCounter("ingress",
				metric.WithIncrement(1000),
				metric.WithVersion(2, 0),
			)
			log.Print("Ingressed (v2) 1000 envelopes")
		}
	}

	return nil
}
