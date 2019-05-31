package diodes

import (
	gendiodes "code.cloudfoundry.org/go-diodes"
	"github.com/cloudfoundry/sonde-go/events"
)

// ManyToOneEnvelope diode is optimal for many writers and a single reader for
// V1 envelopes.
type ManyToOneEnvelope struct {
	d *gendiodes.Poller
}

// NewManyToOneEnvelope returns a new ManyToOneEnvelope diode to be used with
// many writers and a single reader.
func NewManyToOneEnvelope(size int, alerter gendiodes.Alerter) *ManyToOneEnvelope {
	return &ManyToOneEnvelope{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

// Set inserts the given V1 envelope into the diode.
func (d *ManyToOneEnvelope) Set(data *events.Envelope) {
	d.d.Set(gendiodes.GenericDataType(data))
}

// TryNext returns the next V1 envelope to be read from the diode. If the
// diode is empty it will return a nil envelope and false for the bool.
func (d *ManyToOneEnvelope) TryNext() (*events.Envelope, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return (*events.Envelope)(data), true
}

// Next will return the next V1 envelope to be read from the diode. If the
// diode is empty this method will block until anenvelope is available to be
// read.
func (d *ManyToOneEnvelope) Next() *events.Envelope {
	data := d.d.Next()
	return (*events.Envelope)(data)
}
