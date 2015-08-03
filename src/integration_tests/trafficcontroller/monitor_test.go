package integration_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/networkreader"
	"metron/writers/eventunmarshaller"
	"sync/atomic"
)

var _ = Describe("Monitor", func() {
	It("sends uptime metrics", func() {
		logger := loggertesthelper.Logger()
		writer := &fakeWriter{}
		dropsondeUnmarshaller := eventunmarshaller.New(writer, logger)
		dropsondeReader := networkreader.New("localhost:37474", "dropsondeAgentListener", dropsondeUnmarshaller, logger)

		go dropsondeReader.Start()
		defer dropsondeReader.Stop()

		Eventually(func() uint64 { return atomic.LoadUint64(&writer.lastUptime) }, 3).Should(BeNumerically(">", 1))
	})
})

type fakeWriter struct {
	lastUptime uint64
}

func (f *fakeWriter) Write(message *events.Envelope) {
	if message.GetEventType() == events.Envelope_ValueMetric && message.GetValueMetric().GetName() == "Uptime" {
		atomic.StoreUint64(&f.lastUptime, uint64(message.GetValueMetric().GetValue()))
	}
}
