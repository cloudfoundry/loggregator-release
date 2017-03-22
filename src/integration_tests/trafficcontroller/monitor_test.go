package trafficcontroller_test

import (
	"sync/atomic"

	ingress "metron/ingress/v1"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	It("sends uptime metrics", func() {
		writer := &fakeWriter{}

		mockBatcher := newMockEventBatcher()
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		var err error
		dropsondeUnmarshaller := ingress.NewUnMarshaller(writer, mockBatcher)
		dropsondeReader, err := ingress.New("127.0.0.1:37474", "dropsondeAgentListener", dropsondeUnmarshaller)
		Expect(err).NotTo(HaveOccurred())

		go dropsondeReader.StartReading()
		go dropsondeReader.StartWriting()
		defer dropsondeReader.Stop()

		Eventually(func() uint64 { return atomic.LoadUint64(&writer.lastUptime) }, 3).Should(BeNumerically(">", 1))
	})
})
