package varz_forwarder_test

import (
	"github.com/cloudfoundry/dropsonde/events"
	"metron/varz_forwarder"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VarzForwarder", func() {
	var (
		forwarder  *varz_forwarder.VarzForwarder
		metricChan chan *events.Envelope
		outputChan chan *events.Envelope
	)

	BeforeEach(func() {
		forwarder = varz_forwarder.NewVarzForwarder()
		metricChan = make(chan *events.Envelope)
		outputChan = make(chan *events.Envelope, 2)
	})

	var perform = func() {
		go forwarder.Run(metricChan, outputChan)
	}

	Describe("Emit", func() {
		It("includes metrics for each ValueMetric sent in", func() {
			perform()
			metricChan <- metric("origin-1", "metric", 0)
			metricChan <- metric("origin-2", "metric", 0)

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(2))
			Expect(varz.Metrics[0].Name).To(Equal("origin-1.metric"))
			Expect(varz.Metrics[1].Name).To(Equal("origin-2.metric"))
		})

		It("includes metrics for each ValueMetric name in a given origin", func() {
			perform()
			metricChan <- metric("origin", "metric-1", 1)
			metricChan <- metric("origin", "metric-2", 2)

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(2))
			Expect(varz.Metrics[0].Name).To(Equal("origin.metric-1"))
			Expect(varz.Metrics[0].Value).To(BeNumerically("==", 1))
			Expect(varz.Metrics[1].Name).To(Equal("origin.metric-2"))
			Expect(varz.Metrics[1].Value).To(BeNumerically("==", 2))
		})

		It("ignores non-ValueMetric messages", func() {
			perform()

			metricChan <- metric("origin", "metric-1", 0)
			metricChan <- heartbeat()

			var varz instrumentation.Context
			Consistently(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(1))
		})
	})

	Describe("Run", func() {
		It("passes ValueMetrics through", func() {
			perform()
			expectedMetric := metric("origin", "metric", 0)
			metricChan <- expectedMetric

			Eventually(outputChan).Should(Receive(Equal(expectedMetric)))
		})

		It("passes other metrics through", func() {
			perform()
			expectedMetric := heartbeat()
			metricChan <- expectedMetric

			Eventually(outputChan).Should(Receive(Equal(expectedMetric)))
		})
	})
})

func metric(origin, name string, value float64) *events.Envelope {
	return &events.Envelope{
		Origin:      &origin,
		EventType:   events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{Name: &name, Value: &value},
	}
}

func heartbeat() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("heartbeat-origin"),
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: &events.Heartbeat{},
	}
}
