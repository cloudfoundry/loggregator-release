package v2

import (
	"log"
	"metric"
	plumbing "plumbing/v2"
	"time"
)

type Nexter interface {
	Next() *plumbing.Envelope
}

type Writer interface {
	Write(msg *plumbing.Envelope) error
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
	var count int
	lastEmitted := time.Now()
	for {
		envelope := t.nexter.Next()
		err := t.writer.Write(envelope)
		if err != nil {
			metric.IncCounter("dropped",
				metric.WithVersion(2, 0),
				metric.WithTag("direction", "egress"),
			)
			log.Printf("v2 egress dropped: %s", err)
			continue
		}
		count++
		if count%1000 == 0 || time.Since(lastEmitted) > 5*time.Second {
			metric.IncCounter("egress",
				metric.WithIncrement(1000),
				metric.WithVersion(2, 0),
			)
			lastEmitted = time.Now()
			log.Print("egressed (v2) 1000 envelopes")
		}
	}
}
