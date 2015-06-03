package varzforwarder_test

import (
	"metron/varzforwarder"
	"time"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VarzForwarder", func() {
	var (
		forwarder  *varzforwarder.VarzForwarder
		metricChan chan *events.Envelope
		outputChan chan *events.Envelope
	)

	BeforeEach(func() {
		forwarder = varzforwarder.New("test-component", time.Millisecond*100, loggertesthelper.Logger())
		metricChan = make(chan *events.Envelope)
		outputChan = make(chan *events.Envelope, 1024)
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
			Expect(findMetricByName(varz.Metrics, "origin-1.metric")).ToNot(BeNil())
			Expect(findMetricByName(varz.Metrics, "origin-2.metric")).ToNot(BeNil())
		})

		It("keeps track of the dea logging agent total message", func() {
			perform()

			totalLogMessagesSentMetric := metric("dea-logging-agent", "logSenderTotalMessagesRead", 100)
			app1Metrics := metric("dea-logging-agent", "logSenderTotalMessagesRead.appId1", 40)
			app2Metrics := metric("dea-logging-agent", "logSenderTotalMessagesRead.appId2", 60)

			metricChan <- totalLogMessagesSentMetric
			metricChan <- app1Metrics
			metricChan <- app2Metrics

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(3))
			Expect(findMetricByName(varz.Metrics, "dea-logging-agent.logSenderTotalMessagesRead").Value).To(Equal(float64(100)))
		})

		It("includes metrics for each ValueMetric name in a given origin", func() {
			perform()
			metricChan <- metric("origin", "metric-1", 1)
			metricChan <- metric("origin", "metric-2", 2)

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(2))
			metric1 := findMetricByName(varz.Metrics, "origin.metric-1")
			Expect(metric1.Value).To(BeNumerically("==", 1))

			metric2 := findMetricByName(varz.Metrics, "origin.metric-2")
			Expect(metric2.Value).To(BeNumerically("==", 2))
		})

		It("includes metrics for http request count", func() {
			perform()
			metricChan <- httpmetric("origin", 100)
			metricChan <- httpmetric("origin", 199)
			metricChan <- httpmetric("origin", 200)
			metricChan <- httpmetric("origin", 299)
			metricChan <- httpmetric("origin", 300)
			metricChan <- httpmetric("origin", 399)
			metricChan <- httpmetric("origin", 400)
			metricChan <- httpmetric("origin", 499)
			metricChan <- httpmetric("origin", 500)
			metricChan <- httpmetric("origin", 599)

			Eventually(func() []instrumentation.Metric { return forwarder.Emit().Metrics }).Should(HaveLen(6))
			Eventually(func() interface{} { return findMetricByName(forwarder.Emit().Metrics, "origin.requestCount").Value }).Should(BeNumerically("==", 10))

			varz := forwarder.Emit()

			metric := findMetricByName(varz.Metrics, "origin.responseCount1XX")
			Expect(metric.Value).To(BeNumerically("==", 2))

			metric = findMetricByName(varz.Metrics, "origin.responseCount2XX")
			Expect(metric.Value).To(BeNumerically("==", 2))

			metric = findMetricByName(varz.Metrics, "origin.responseCount3XX")
			Expect(metric.Value).To(BeNumerically("==", 2))

			metric = findMetricByName(varz.Metrics, "origin.responseCount4XX")
			Expect(metric.Value).To(BeNumerically("==", 2))

			metric = findMetricByName(varz.Metrics, "origin.responseCount5XX")
			Expect(metric.Value).To(BeNumerically("==", 2))
		})

		It("increments value for each CounterEvent name in a given origin", func() {
			perform()
			metricChan <- counterEvent("origin-0", "metric-1", 1)
			metricChan <- counterEvent("origin-0", "metric-1", 3)
			metricChan <- counterEvent("origin-1", "metric-1", 1)

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(2))

			metric1 := findMetricByName(varz.Metrics, "origin-0.metric-1")
			Expect(metric1.Value).To(BeNumerically("==", 4))

			metric2 := findMetricByName(varz.Metrics, "origin-1.metric-1")
			Expect(metric2.Value).To(BeNumerically("==", 1))
		})

		It("includes the VM name as a tag on each metric", func() {
			perform()
			metricChan <- metric("origin", "metric", 1)

			var varz instrumentation.Context
			Eventually(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(1))
			metric := findMetricByName(varz.Metrics, "origin.metric")
			Expect(metric.Tags["component"]).To(Equal("test-component"))
		})

		It("ignores non-ValueMetric messages", func() {
			perform()

			metricChan <- metric("origin", "metric-1", 0)
			metricChan <- heartbeat("origin")

			var varz instrumentation.Context
			Consistently(func() []instrumentation.Metric { varz = forwarder.Emit(); return varz.Metrics }).Should(HaveLen(1))
		})

		It("no longer emits metrics when the origin TTL expires", func() {
			perform()

			metricChan <- metric("origin", "metric-X", 0)

			Eventually(func() []instrumentation.Metric { return forwarder.Emit().Metrics }).ShouldNot(HaveLen(0))

			time.Sleep(time.Millisecond * 200)

			Expect(forwarder.Emit().Metrics).To(HaveLen(0))
		})

		It("still emits metrics after origin TTL if new events were received", func() {
			perform()

			metricChan <- metric("origin", "metric-X", 0)

			stopHeartbeats := make(chan struct{})
			heartbeatsStopped := make(chan struct{})
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
					case <-stopHeartbeats:
						close(heartbeatsStopped)
						return
					}
					metricChan <- heartbeat("origin")
					<-outputChan
				}
			}()

			Eventually(func() []instrumentation.Metric { return forwarder.Emit().Metrics }).ShouldNot(HaveLen(0))

			time.Sleep(time.Millisecond * 200)

			Expect(forwarder.Emit().Metrics).ToNot(HaveLen(0))
			close(stopHeartbeats)
			<-heartbeatsStopped
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
			expectedMetric := heartbeat("origin")
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

func counterEvent(origin, name string, delta uint64) *events.Envelope {
	return &events.Envelope{
		Origin:       &origin,
		EventType:    events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{Name: &name, Delta: proto.Uint64(delta)},
	}
}

func heartbeat(origin string) *events.Envelope {
	return &events.Envelope{
		Origin:    &origin,
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: &events.Heartbeat{},
	}
}

func httpmetric(origin string, status int32) *events.Envelope {
	return &events.Envelope{
		Origin:        &origin,
		EventType:     events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{StatusCode: &status},
	}
}

func findMetricByName(metrics []instrumentation.Metric, metricName string) *instrumentation.Metric {
	for _, metric := range metrics {
		if metric.Name == metricName {
			return &metric
		}
	}

	return nil
}
