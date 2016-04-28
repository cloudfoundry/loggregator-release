// +build linux

package monitor_test

import (
	"monitor"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const initialFDs = 5

var _ = Describe("OpenFD", func() {
	var (
		openFDMonitor *monitor.OpenFileDescriptor
		logger        *gosteno.Logger
	)

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		interval := 100 * time.Millisecond
		logger = loggertesthelper.Logger()

		openFDMonitor = monitor.NewOpenFD(interval, logger)
		go openFDMonitor.Start()
	})

	AfterEach(func() {
		openFDMonitor.Stop()
	})

	It("emits a metric with the number of open file handles", func() {
		Eventually(func() int { return len(fakeEventEmitter.GetMessages()) }).Should(BeNumerically(">", 0))
		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("OpenFileDescriptor"),
			Value: proto.Float64(initialFDs),
			Unit:  proto.String("File"),
		}))

		listener, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		defer listener.Close()

		Eventually(fakeEventEmitter.GetMessages).Should(ContainElement(
			fake.Message{
				Origin: "MonitorTest",
				Event: &events.ValueMetric{
					Name:  proto.String("OpenFileDescriptor"),
					Value: proto.Float64(initialFDs + 2),
					Unit:  proto.String("File"),
				},
			},
		))
	})
})
