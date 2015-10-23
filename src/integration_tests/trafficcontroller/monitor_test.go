package integration_test

import (
	"metron/networkreader"
	"metron/writers/eventunmarshaller"
	"sync/atomic"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	It("sends uptime metrics", func() {
		logger := loggertesthelper.Logger()
		writer := &fakeWriter{}
		dropsondeUnmarshaller := eventunmarshaller.New(writer, logger)
		dropsondeReader, err := networkreader.New("127.0.0.1:37474", "dropsondeAgentListener", dropsondeUnmarshaller, logger)
		Expect(err).NotTo(HaveOccurred())

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
