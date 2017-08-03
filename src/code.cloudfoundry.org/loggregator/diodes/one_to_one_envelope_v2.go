package diodes

import (
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	gendiodes "github.com/cloudfoundry/diodes"
)

// OneToOneEnvelopeV2 diode is optimized for a single writer and a single reader
type OneToOneEnvelopeV2 struct {
	d *gendiodes.Poller
}

// NewOneToOneEnvelopeV2 initializes a new one to one diode for V2 envelopes
// of a given size and alerter. The alerter is called whenever data is dropped
// with an integer representing the number of V2 envelopes that were dropped.
func NewOneToOneEnvelopeV2(size int, alerter gendiodes.Alerter, opts ...gendiodes.PollerConfigOption) *OneToOneEnvelopeV2 {
	return &OneToOneEnvelopeV2{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(size, alerter), opts...),
	}
}

// Set inserts the given V2 envelope into the diode.
func (d *OneToOneEnvelopeV2) Set(data *v2.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

// TryNext returns the next V2 envelope to be read from the diode. If the
// diode is empty it will return a nil envelope and false for the bool.
func (d *OneToOneEnvelopeV2) TryNext() (*v2.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*v2.Envelope)(data), true
}

// Next will return the next V2 envelope to be read from the diode. If the
// diode is empty this method will block until anenvelope is available to be
// read.
func (d *OneToOneEnvelopeV2) Next() *v2.Envelope {
	data := d.d.Next()
	return (*v2.Envelope)(data)
}
