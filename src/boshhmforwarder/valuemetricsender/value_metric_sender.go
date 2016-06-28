package valuemetricsender

import (
	"time"

	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const ForwarderOrigin = "bosh-hm-forwarder"

type valueMetricSender struct{}

func NewValueMetricSender() *valueMetricSender {
	return &valueMetricSender{}
}

func (s *valueMetricSender) SendValueMetric(deployment, job, index, eventName string, secondsSinceEpoch int64, value float64, units string) error {
	envelope := events.Envelope{
		Origin:     proto.String(ForwarderOrigin),
		EventType:  events.Envelope_ValueMetric.Enum(),
		Timestamp:  proto.Int64(secondsSinceEpoch * int64(time.Second)),
		Deployment: proto.String(deployment),
		Job:        proto.String(job),
		Index:      proto.String(index),
		Ip:         proto.String(""),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String(eventName),
			Value: proto.Float64(value),
			Unit:  proto.String(units),
		},
	}
	return envelopes.SendEnvelope(&envelope)
}
