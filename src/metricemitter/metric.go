package metricemitter

import (
	"fmt"
	v2 "plumbing/v2"
	"sync/atomic"
)

type metric struct {
	client *client
	name   string
	tags   map[string]*v2.Value
	count  uint64
}

type MetricOption func(*metric)

func newMetric(name string, opts ...MetricOption) *metric {
	m := &metric{
		name: name,
		tags: make(map[string]*v2.Value),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *metric) Increment(c uint64) {
	atomic.AddUint64(&m.count, c)
}

func (m *metric) GetDelta() uint64 {
	return atomic.SwapUint64(&m.count, 0)
}

func WithVersion(major, minor uint) MetricOption {
	return WithTags(map[string]string{
		"metric_version": fmt.Sprintf("%d.%d", major, minor),
	})
}

func WithTags(tags map[string]string) MetricOption {
	return func(m *metric) {
		for k, v := range tags {
			m.tags[k] = &v2.Value{
				Data: &v2.Value_Text{
					Text: v,
				},
			}
		}
	}
}
