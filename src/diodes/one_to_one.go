package diodes

import gendiodes "github.com/cloudfoundry/diodes"

// OneToOne diode is optimized for a single writer and a single reader
type OneToOne struct {
	d *gendiodes.Poller
}

func NewOneToOne(size int, alerter gendiodes.Alerter) *OneToOne {
	return &OneToOne{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(size, alerter)),
	}
}

func (d *OneToOne) Set(data []byte) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

func (d *OneToOne) TryNext() ([]byte, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return *(*[]byte)(data), true
}

func (d *OneToOne) Next() []byte {
	data := d.d.Next()
	return *(*[]byte)(data)
}
