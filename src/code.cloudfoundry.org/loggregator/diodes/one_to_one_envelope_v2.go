package diodes

import (
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	gendiodes "github.com/cloudfoundry/diodes"
)

// OneToOneEnvelopeV2 diode is optimized for a single writer and a single reader
type OneToOneEnvelopeV2 struct {
	d *gendiodes.Poller
}

func NewOneToOneEnvelopeV2(size int, alerter gendiodes.Alerter, opts ...gendiodes.PollerConfigOption) *OneToOneEnvelopeV2 {
	return &OneToOneEnvelopeV2{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(size, alerter), opts...),
	}
}

func (d *OneToOneEnvelopeV2) Set(data *v2.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

func (d *OneToOneEnvelopeV2) TryNext() (*v2.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*v2.Envelope)(data), true
}

func (d *OneToOneEnvelopeV2) Next() *v2.Envelope {
	data := d.d.Next()
	return (*v2.Envelope)(data)
}
