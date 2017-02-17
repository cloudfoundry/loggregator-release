package metric

import (
	"fmt"
	"time"

	v2 "plumbing/v2"
)

type IncrementOpt func(*incrementOption)

type incrementOption struct {
	delta uint64
	tags  map[string]string
}

func WithIncrement(delta uint64) func(*incrementOption) {
	return func(i *incrementOption) {
		i.delta = delta
	}
}

func WithVersion(major, minor uint) func(*incrementOption) {
	return func(i *incrementOption) {
		i.tags["metric_version"] = fmt.Sprintf("%d.%d", major, minor)
	}
}

func WithTag(name, value string) func(*incrementOption) {
	return func(i *incrementOption) {
		i.tags[name] = value
	}
}

func IncCounter(name string, options ...IncrementOpt) {
	if batchBuffer == nil {
		return
	}

	incConf := &incrementOption{
		delta: 1,
		tags:  make(map[string]string),
	}

	for _, opt := range options {
		opt(incConf)
	}

	tags := make(map[string]*v2.Value)
	for k, v := range incConf.tags {
		tags[k] = &v2.Value{
			Data: &v2.Value_Text{
				Text: v,
			},
		}
	}

	for k, v := range conf.tags {
		tags[k] = &v2.Value{
			Data: &v2.Value_Text{
				Text: v,
			},
		}
	}

	e := &v2.Envelope{
		SourceId:  conf.sourceUUID,
		Timestamp: time.Now().UnixNano(),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: fmt.Sprintf("%s.%s", conf.tags["prefix"], name),
				Value: &v2.Counter_Delta{
					Delta: incConf.delta,
				},
			},
		},
		Tags: tags,
	}

	batchBuffer.Set(e)
}

func runBatcher() {
	ticker := time.NewTicker(conf.batchInterval)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		s := sender
		mu.Unlock()

		if s == nil {
			continue
		}

		for _, e := range aggregateCounters() {
			s.Send(e)
		}
	}
}

func aggregateCounters() map[string]*v2.Envelope {
	m := make(map[string]*v2.Envelope)
	for {
		envelope, ok := batchBuffer.TryNext()
		if !ok {
			break
		}

		existingEnvelope, ok := m[envelope.GetCounter().Name]
		if !ok {
			existingEnvelope = envelope
			m[envelope.GetCounter().Name] = existingEnvelope
			continue
		}

		existingEnvelope.GetCounter().GetValue().(*v2.Counter_Delta).Delta += envelope.GetCounter().GetDelta()
	}

	return m
}
