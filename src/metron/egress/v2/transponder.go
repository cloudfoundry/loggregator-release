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
		err := t.writer.Write(envelope)
		if err != nil {
			metric.IncCounter("dropped") // TODO: add "egress" tag
			continue
		}
		metric.IncCounter("egress")
	}
}
