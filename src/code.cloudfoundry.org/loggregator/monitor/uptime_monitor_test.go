package monitor_test

import (
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/monitor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
)

const (
	interval = 100 * time.Millisecond
)

var _ = Describe("Uptime", func() {
	var (
		uptime *monitor.Uptime
		wg     sync.WaitGroup
	)

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		uptime = monitor.NewUptime(interval)
		wg.Add(1)
		go func() {
			defer wg.Done()
			uptime.Start()
		}()
	})

	Context("stops automatically", func() {

		AfterEach(func() {
			uptime.Stop()
			wg.Wait()
		})

		var fetchValueMetrics = func() []*events.ValueMetric {
			var results []*events.ValueMetric
			for _, m := range fakeEventEmitter.GetMessages() {
				e, ok := m.Event.(*events.ValueMetric)
				if ok {
					results = append(results, e)
				}
			}

			return results
		}

		var fetchNumOfMetrics = func() int {
			return len(fetchValueMetrics())
		}

		var fetchLatestValue = func() float64 {
			metrics := fetchValueMetrics()
			if len(metrics) <= 0 {
				return 0
			}

			return metrics[len(metrics)-1].GetValue()
		}

		It("returns a value metric containing uptime after specified time", func() {
			Eventually(fetchNumOfMetrics).Should(BeNumerically(">", 0))
			metric := fetchValueMetrics()[0]
			Expect(metric.GetName()).To(Equal("Uptime"))
			Expect(metric.GetUnit()).To(Equal("seconds"))
		})

		It("reports increasing uptime value", func() {
			Eventually(fetchLatestValue, 3).Should(BeNumerically(">", 0))
		})
	})

	It("stops the monitor and respective ticker", func() {
		Eventually(func() int { return len(fakeEventEmitter.GetMessages()) }).Should(BeNumerically(">=", 1))

		uptime.Stop()

		current := getLatestUptime()
		Consistently(getLatestUptime, 2).Should(Equal(current))
	})
})

func getLatestUptime() float64 {
	lastMsgIndex := len(fakeEventEmitter.GetMessages()) - 1
	return *fakeEventEmitter.GetMessages()[lastMsgIndex].Event.(*events.ValueMetric).Value
}
