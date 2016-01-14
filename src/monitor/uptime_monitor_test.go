package monitor_test

import (
	"monitor"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fakeEventEmitter *fake.FakeEventEmitter
	uptimeMonitor    monitor.Monitor
)

const (
	interval = 100 * time.Millisecond
)

var _ = Describe("UptimeMonitor", func() {
	BeforeEach(func() {
		fakeEventEmitter = fake.NewFakeEventEmitter("MonitorTest")
		sender := metric_sender.NewMetricSender(fakeEventEmitter)
		batcher := metricbatcher.New(sender, 100*time.Millisecond)

		metrics.Initialize(sender, batcher)

		uptimeMonitor = monitor.NewUptimeMonitor(interval)
		go uptimeMonitor.Start()
	})

	AfterEach(func() {
		fakeEventEmitter.Close()
	})

	Context("stops automatically", func() {

		AfterEach(func() {
			uptimeMonitor.Stop()
		})

		PIt("returns a value metric containing uptime after specified time", func() {
			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
				Name:  proto.String("Uptime"),
				Value: proto.Float64(0),
				Unit:  proto.String("seconds"),
			}))
		})

		PIt("reports increasing uptime value", func() {
			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
				Name:  proto.String("Uptime"),
				Value: proto.Float64(0),
				Unit:  proto.String("seconds"),
			}))

			Eventually(getLatestUptime).Should(Equal(1.0))
		})
	})

	It("stops the monitor and respective ticker", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))

		uptimeMonitor.Stop()

		current := getLatestUptime()
		Consistently(getLatestUptime, 2).Should(Equal(current))
	})
})

func getLatestUptime() float64 {
	lastMsgIndex := len(fakeEventEmitter.GetMessages()) - 1
	return *fakeEventEmitter.GetMessages()[lastMsgIndex].Event.(*events.ValueMetric).Value
}
