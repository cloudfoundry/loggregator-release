package diodes

import (
	gendiodes "github.com/cloudfoundry/diodes"
	"github.com/cloudfoundry/sonde-go/events"
)

// ManyToOneEnvelope diode is optimal for many writers and a single
// reader.
type ManyToOneEnvelope struct {
	d *gendiodes.Poller
}

func NewManyToOneEnvelope(size int, alerter gendiodes.Alerter) *ManyToOneEnvelope {
	return &ManyToOneEnvelope{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

func (d *ManyToOneEnvelope) Set(data *events.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

func (d *ManyToOneEnvelope) TryNext() (*events.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*events.Envelope)(data), true
}

func (d *ManyToOneEnvelope) Next() *events.Envelope {
	data := d.d.Next()
	return (*events.Envelope)(data)
}
