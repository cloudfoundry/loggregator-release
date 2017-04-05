package v2

import (
	"log"
	"metric"
	plumbing "plumbing/v2"
	"time"
)

type Nexter interface {
	TryNext() (*plumbing.Envelope, bool)
}

type Writer interface {
	Write(msgs []*plumbing.Envelope) error
}

type Transponder struct {
	nexter        Nexter
	writer        Writer
	tags          map[string]string
	batchSize     int
	batchInterval time.Duration
}

func NewTransponder(
	n Nexter,
	w Writer,
	tags map[string]string,
	batchSize int,
	batchInterval time.Duration,
) *Transponder {
	return &Transponder{
		nexter:        n,
		writer:        w,
		tags:          tags,
		batchSize:     batchSize,
		batchInterval: batchInterval,
	}
}

func (t *Transponder) Start() {
	var count uint64
	lastEmitted := time.Now()

	var batch []*plumbing.Envelope
	lastSent := time.Now()

	for {
		envelope, ok := t.nexter.TryNext()
		if !ok && !t.batchReady(batch, lastSent) {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if ok {
			t.addTags(envelope)
			batch = append(batch, envelope)
		}

		if !t.batchReady(batch, lastSent) {
			continue
		}

		err := t.writer.Write(batch)
		batch = nil
		lastSent = time.Now()
		if err != nil {
			// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
			// dropped when failing to write to Dopplers v2 API
			metric.IncCounter("dropped",
				metric.WithVersion(2, 0),
				metric.WithTag("direction", "egress"),
			)
			log.Printf("v2 egress dropped: %s", err)
			continue
		}

		count++
		if count >= 1000 || time.Since(lastEmitted) > 5*time.Second {
			// metric-documentation-v2: (loggregator.metron.egress) Number of messages
			// written to Doppler's v2 API
			metric.IncCounter("egress",
				metric.WithIncrement(count),
				metric.WithVersion(2, 0),
			)
			lastEmitted = time.Now()
			log.Printf("egressed (v2) %d envelopes", count)
			count = 0
		}
	}
}

func (t *Transponder) batchReady(batch []*plumbing.Envelope, lastSent time.Time) bool {
	if len(batch) == 0 {
		return false
	}

	return len(batch) >= t.batchSize || time.Since(lastSent) >= t.batchInterval
}

func (t *Transponder) addTags(e *plumbing.Envelope) {
	if e.Tags == nil {
		e.Tags = make(map[string]*plumbing.Value)
	}
	for k, v := range t.tags {
		if _, ok := e.Tags[k]; !ok {
			e.Tags[k] = &plumbing.Value{
				Data: &plumbing.Value_Text{
					Text: v,
				},
			}
		}
	}
}
