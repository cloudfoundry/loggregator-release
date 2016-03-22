package doppler_test

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

	var (
		writer          *fakeWriter
		dropsondeReader *networkreader.NetworkReader
	)

	BeforeEach(func() {
		logger := loggertesthelper.Logger()
		writer = &fakeWriter{}
		var err error
		dropsondeUnmarshaller := eventunmarshaller.New(writer, logger)
		dropsondeReader, err = networkreader.New("127.0.0.1:37474", "dropsondeAgentListener", dropsondeUnmarshaller, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Uptime", func() {
		It("sends uptime metrics", func() {
			defer dropsondeReader.Stop()
			go dropsondeReader.Start()

			Eventually(func() uint64 { return atomic.LoadUint64(&writer.lastUptime) }, 3).Should(BeNumerically(">", 1))
		})
	})

	Context("OpenFileDescriptors", func() {
		It("sends openfile metrics", func() {
			defer dropsondeReader.Stop()
			go dropsondeReader.Start()

			Eventually(func() uint63 { return atomic.LoadUint64(&writer.openFileDescriptors) }, 3).Should(BeNumerically(">", 3))
		})
	})
})

type fakeWriter struct {
	openFileDescriptors uint64
	lastUptime          uint64
}

func (f *fakeWriter) Write(message *events.Envelope) {
	if message.GetEventType() == events.Envelope_ValueMetric {

		switch message.GetValueMetric().GetName() {
		case "Uptime":
			atomic.StoreUint64(&f.lastUptime, uint64(message.GetValueMetric().GetValue()))
		case "OpenFileDescriptor":
			atomic.StoreUint64(&f.openFileDescriptors, uint64(message.GetValueMetric().GetValue()))
		}
	}
}
