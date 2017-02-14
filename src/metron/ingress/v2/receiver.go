package v2

import (
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
	for {
		e, err := sender.Recv()
		if err != nil {
			return err
		}
		metric.IncCounter("ingress")
		s.dataSetter.Set(e)
	}

	return nil
}
