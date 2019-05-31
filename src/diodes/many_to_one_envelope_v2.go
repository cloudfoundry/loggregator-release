package diodes

import (
	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

// ManyToOneEnvelopeV2 diode is optimal for many writers and a single reader for
// V2 envelopes.
type ManyToOneEnvelopeV2 struct {
	d *gendiodes.Poller
}

// NewManyToOneEnvelopeV2 returns a new ManyToOneEnvelopeV2 diode to be used
// with many writers and a single reader.
func NewManyToOneEnvelopeV2(size int, alerter gendiodes.Alerter) *ManyToOneEnvelopeV2 {
	return &ManyToOneEnvelopeV2{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

// Set inserts the given V2 envelope into the diode.
func (d *ManyToOneEnvelopeV2) Set(data *loggregator_v2.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

// TryNext returns the next V2 envelope to be read from the diode. If the
// diode is empty it will return a nil envelope and false for the bool.
func (d *ManyToOneEnvelopeV2) TryNext() (*loggregator_v2.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*loggregator_v2.Envelope)(data), true
}

// Next will return the next V2 envelope to be read from the diode. If the
// diode is empty this method will block until anenvelope is available to be
// read.
func (d *ManyToOneEnvelopeV2) Next() *loggregator_v2.Envelope {
	data := d.d.Next()
	return (*loggregator_v2.Envelope)(data)
}
