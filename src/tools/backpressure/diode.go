package main

import (
	"time"

	gendiodes "github.com/cloudfoundry/diodes"
)

type diode struct {
	d *gendiodes.Poller
}

func newDiode() *diode {
	alerter := gendiodes.AlertFunc(func(int) {})
	return &diode{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(10000, alerter)),
	}
}

func (d *diode) Set(data time.Duration) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

func (d *diode) TryNext() (time.Duration, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return 0, ok
	}
	return *(*time.Duration)(data), true
}

func (d *diode) Next() time.Duration {
	data := d.d.Next()
	return *(*time.Duration)(data)
}
