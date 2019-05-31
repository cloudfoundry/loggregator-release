package diodes

import gendiodes "code.cloudfoundry.org/go-diodes"

// ManyToOne diode is optimal for many writers and a single reader for slices
// of bytes.
type ManyToOne struct {
	d *gendiodes.Poller
}

// NewManyToOne initializes a new many to one diode of a given size and alerter.
// The alerter is called whenever data is dropped with an integer representing
// the number of byte slices that were dropped.
func NewManyToOne(size int, alerter gendiodes.Alerter) *ManyToOne {
	return &ManyToOne{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

// Set inserts the given data into the diode.
func (d *ManyToOne) Set(data []byte) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

// TryNext returns the next item to be read from the diode. If the diode is
// empty it will return a nil slice of bytes and false for the bool.
func (d *ManyToOne) TryNext() ([]byte, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return *(*[]byte)(data), true
}

// Next will return the next item to be read from the diode. If the diode is
// empty this method will block until an item is available to be read.
func (d *ManyToOne) Next() []byte {
	data := d.d.Next()
	return *(*[]byte)(data)
}
