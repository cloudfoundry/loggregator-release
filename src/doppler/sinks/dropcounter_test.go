package sinks_test

import (
	"doppler/sinks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropCounter", func() {
	It("updates its internal counter and returns a properly-formatted Metric object", func() {
		metricChan := make(chan int64, 1)
		dc := sinks.NewDropCounter("appID", "drainURL", metricChan)

		dc.UpdateDroppedMessageCount(20)

		m := dc.GetInstrumentationMetric()
		Expect(m).To(Equal(sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "appID", "drainUrl": "drainURL"}, Value: int64(20)}))
		Expect(metricChan).To(Receive(Equal(int64(20))))

		dc.UpdateDroppedMessageCount(17)

		m = dc.GetInstrumentationMetric()
		Expect(m).To(Equal(sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "appID", "drainUrl": "drainURL"}, Value: int64(37)}))
		Expect(metricChan).To(Receive(Equal(int64(17))))
	})
})
