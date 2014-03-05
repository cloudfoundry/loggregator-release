package metrics

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type MessageRouterMetrics struct {
	UnmarshalledInParseEnvelopes    uint
	UnmarshalErrorsInParseEnvelopes uint
	DroppedInParseEnvelopes         uint
	ReceivedMessages				uint64
}

func (messageRouterMetrics *MessageRouterMetrics) Emit() instrumentation.Context {
	data := []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfMessagesUnmarshalledInParseEnvelopes", Value: messageRouterMetrics.UnmarshalledInParseEnvelopes},
		instrumentation.Metric{Name: "numberOfMessagesUnmarshalErrorsInParseEnvelopes", Value: messageRouterMetrics.UnmarshalErrorsInParseEnvelopes},
		instrumentation.Metric{Name: "numberOfMessagesDroppedInParseEnvelopes", Value: messageRouterMetrics.DroppedInParseEnvelopes},
		instrumentation.Metric{Name: "receivedMessages", Value: messageRouterMetrics.ReceivedMessages},
	}

	return instrumentation.Context{
		Name:    "httpServer",
		Metrics: data,
	}
}
