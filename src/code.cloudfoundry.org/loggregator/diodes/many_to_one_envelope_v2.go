package diodes

import (
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	gendiodes "github.com/cloudfoundry/diodes"
)

// ManyToOneEnvelopeV2 diode is optimal for many writers and a single
// reader.
type ManyToOneEnvelopeV2 struct {
	d *gendiodes.Poller
}

func NewManyToOneEnvelopeV2(size int, alerter gendiodes.Alerter) *ManyToOneEnvelopeV2 {
	return &ManyToOneEnvelopeV2{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

func (d *ManyToOneEnvelopeV2) Set(data *v2.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

func (d *ManyToOneEnvelopeV2) TryNext() (*v2.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*v2.Envelope)(data), true
}

func (d *ManyToOneEnvelopeV2) Next() *v2.Envelope {
	data := d.d.Next()
	return (*v2.Envelope)(data)
}
