package diodes

import gendiodes "github.com/cloudfoundry/diodes"

// ManyToOne diode is optimal for many writers and a single
// reader.
type ManyToOne struct {
	d *gendiodes.Poller
}

func NewManyToOne(size int, alerter gendiodes.Alerter) *ManyToOne {
	return &ManyToOne{
		d: gendiodes.NewPoller(gendiodes.NewManyToOne(size, alerter)),
	}
}

func (d *ManyToOne) Set(data []byte) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

func (d *ManyToOne) TryNext() ([]byte, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return nil, ok
	}

	return *(*[]byte)(data), true
}

func (d *ManyToOne) Next() []byte {
	data := d.d.Next()
	return *(*[]byte)(data)
}
