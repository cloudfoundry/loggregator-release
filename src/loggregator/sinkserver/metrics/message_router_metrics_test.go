package metrics_test

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinkserver/metrics"
)

var _ = Describe("MessageRouterMetrics", func() {

	var expectedMetrics = instrumentation.Context{
		Name: "httpServer",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "numberOfMessagesUnmarshalledInParseEnvelopes", Value: 0},
			instrumentation.Metric{Name: "numberOfMessagesUnmarshalErrorsInParseEnvelopes", Value: 0},
			instrumentation.Metric{Name: "numberOfMessagesDroppedInParseEnvelopes", Value: 0},
			instrumentation.Metric{Name: "receivedMessages", Value: 0},
		},
	}

	It("should emit metrics", func() {
		routerMetrics := new(metrics.MessageRouterMetrics)
		actualMetrics := routerMetrics.Emit()
		Expect(actualMetrics.Name).To(Equal(expectedMetrics.Name))
		Expect(len(actualMetrics.Metrics)).To(Equal(len(expectedMetrics.Metrics)))
		for i, aMetric := range actualMetrics.Metrics {
			expectedMetric := expectedMetrics.Metrics[i]
			Expect(aMetric.Name).To(Equal(expectedMetric.Name))
			Expect(aMetric.Value).To(BeEquivalentTo(expectedMetric.Value))
			Expect(aMetric.Tags).To(BeNil())
		}
	})
})
