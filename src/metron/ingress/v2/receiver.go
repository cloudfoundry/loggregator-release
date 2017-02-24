package v2

import (
	"metric"
	"log"
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
	for {
		e, err := sender.Recv()
		if err != nil {
		        log.Printf("Failed to receive data: %s", err)
			return err
		}
		metric.IncCounter("ingress", metric.WithVersion(2, 0))
		s.dataSetter.Set(e)
	}

	return nil
}
