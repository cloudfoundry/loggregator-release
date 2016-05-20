package trafficcontroller_test

import (
	"metron/networkreader"
	"metron/writers/eventunmarshaller"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var _ = Describe("Monitor", func() {
	It("sends uptime metrics", func() {
		logger := loggertesthelper.Logger()
		writer := &fakeWriter{}
		mockBatcher := newMockEventBatcher()
		mockChainer := newMockBatchCounterChainer()

		stop := make(chan struct{})
		defer close(stop)
		go func() {
			for {
				select {
				case <-mockBatcher.BatchIncrementCounterCalled:
					<-mockBatcher.BatchIncrementCounterInput.Name
				case <-mockBatcher.BatchCounterCalled:
					<-mockBatcher.BatchCounterInput.Name
					mockBatcher.BatchCounterOutput.Chainer <- mockChainer
				case <-mockChainer.SetTagCalled:
					<-mockChainer.SetTagInput.Key
					<-mockChainer.SetTagInput.Value
					mockChainer.SetTagOutput.Ret0 <- mockChainer
				case <-mockChainer.IncrementCalled:
				case <-mockChainer.AddCalled:
					<-mockChainer.AddInput.Value
				case <-stop:
					return
				}
			}
		}()
		dropsondeUnmarshaller := eventunmarshaller.New(writer, mockBatcher, logger)
		dropsondeReader, err := networkreader.New("127.0.0.1:37474", "dropsondeAgentListener", dropsondeUnmarshaller, logger)
		Expect(err).NotTo(HaveOccurred())

		go dropsondeReader.Start()
		defer dropsondeReader.Stop()

		Eventually(func() uint64 { return atomic.LoadUint64(&writer.lastUptime) }, 3).Should(BeNumerically(">", 1))
	})
})
