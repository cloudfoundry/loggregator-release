package metrics

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type MessageRouterMetrics struct {
	UnmarshalledInParseEnvelopes    uint
	UnmarshalErrorsInParseEnvelopes uint
	DroppedInParseEnvelopes         uint
}

func (messageRouterMetrics *MessageRouterMetrics) Emit() instrumentation.Context {
	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfMessagesUnmarshalledInParseEnvelopes", Value: messageRouterMetrics.UnmarshalledInParseEnvelopes},
		instrumentation.Metric{Name: "numberOfMessagesUnmarshalErrorsInParseEnvelopes", Value: messageRouterMetrics.UnmarshalErrorsInParseEnvelopes},
		instrumentation.Metric{Name: "numberOfMessagesDroppedInParseEnvelopes", Value: messageRouterMetrics.DroppedInParseEnvelopes},
	}

	return instrumentation.Context{
		Name:    "httpServer",
		Metrics: data,
	}
}
