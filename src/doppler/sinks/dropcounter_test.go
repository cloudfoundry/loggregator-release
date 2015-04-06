package sinks_test
import (
    "doppler/sinks"

    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

var _ = Describe("DropCounter", func() {
    It("updates its internal counter and returns a properly-formatted Metric object", func() {
        dc := sinks.NewDropCounter("appID", "drainURL")

        dc.UpdateDroppedMessageCount(20)

        m := dc.GetInstrumentationMetric()
        Expect(m).To(Equal(sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "appID", "drainUrl": "drainURL"}, Value: int64(20)}))

        dc.UpdateDroppedMessageCount(17)

        m = dc.GetInstrumentationMetric()
        Expect(m).To(Equal(sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "appID", "drainUrl": "drainURL"}, Value: int64(37)}))
    })
})
