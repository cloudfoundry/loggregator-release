package egress

import (
	"metric"
	v2 "plumbing/v2"
)

type Nexter interface {
	Next() *v2.Envelope
}

type Writer interface {
	Write(msg *v2.Envelope) error
}

type Transponder struct {
	nexter Nexter
	writer Writer
}

func NewTransponder(n Nexter, w Writer) *Transponder {
	return &Transponder{
		nexter: n,
		writer: w,
	}
}

func (t *Transponder) Start() {
	for {
		envelope := t.nexter.Next()
		t.writer.Write(envelope)
		metric.IncCounter("egress")
	}
}
