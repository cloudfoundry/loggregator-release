package v2

import (
	"metric"
	"plumbing/conversion"
	plumbing "plumbing/v2"

	"github.com/cloudfoundry/sonde-go/events"
)

type DopplerIngress_SenderServer interface {
	plumbing.DopplerIngress_SenderServer
}

type DataSetter interface {
	Set(data *events.Envelope)
}

type Ingestor struct {
	envelopeBuffer DataSetter
}

func NewIngestor(envelopeBuffer DataSetter) *Ingestor {
	return &Ingestor{
		envelopeBuffer: envelopeBuffer,
	}
}

func (i Ingestor) Sender(s plumbing.DopplerIngress_SenderServer) error {
	var count int
	for {
		v2e, err := s.Recv()
		if err != nil {
			return err
		}

		v1e := conversion.ToV1(v2e)
		if v1e == nil || v1e.EventType == nil {
			continue
		}

		count++
		if count%1000 == 0 {
			metric.IncCounter("ingress",
				metric.WithIncrement(1000),
				metric.WithVersion(2, 0),
			)
		}
		i.envelopeBuffer.Set(v1e)
	}
}
