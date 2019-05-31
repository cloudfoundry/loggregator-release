package diodes

import gendiodes "code.cloudfoundry.org/go-diodes"

// OneToOne diode is optimized for a single writer and a single reader for
// byte slices.
type OneToOne struct {
	d *gendiodes.Poller
}

// NewOneToOne initializes a new one to one diode of a given size and alerter.
// The alerter is called whenever data is dropped with an integer representing
// the number of byte slices that were dropped.
func NewOneToOne(size int, alerter gendiodes.Alerter) *OneToOne {
	return &OneToOne{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(size, alerter)),
	}
}

// Set inserts the given data into the diode.
func (d *OneToOne) Set(data []byte) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

// TryNext returns the next item to be read from the diode. If the diode is
// empty it will return a nil slice of bytes and false for the bool.
func (d *OneToOne) TryNext() ([]byte, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return *(*[]byte)(data), true
}

// Next will return the next item to be read from the diode. If the diode is
// empty this method will block until an item is available to be read.
func (d *OneToOne) Next() []byte {
	data := d.d.Next()
	return *(*[]byte)(data)
}
