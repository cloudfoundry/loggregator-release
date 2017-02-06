package doppler_test

import (
	egress "metron/egress/v1"
	ingress "metron/ingress/v1"
	"sync/atomic"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Uptime Monitor", func() {
	var (
		writer          *fakeWriter
		dropsondeReader *ingress.NetworkReader
	)

	BeforeEach(func() {
		writer = &fakeWriter{}

		mockBatcher := newMockEventBatcher()
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		var err error
		dropsondeUnmarshaller := egress.NewUnMarshaller(writer, mockBatcher)
		dropsondeReader, err = ingress.New(
			"localhost:37474",
			"dropsondeAgentListener",
			dropsondeUnmarshaller,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Uptime", func() {
		It("sends uptime metrics", func() {
			go dropsondeReader.StartWriting()
			go dropsondeReader.StartReading()
			defer dropsondeReader.Stop()

			Eventually(func() uint64 {
				return atomic.LoadUint64(&writer.lastUptime)
			}, 3).Should(BeNumerically(">", 1))
		})
	})
})
