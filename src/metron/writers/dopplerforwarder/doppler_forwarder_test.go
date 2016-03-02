package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerForwarder", func() {
	var (
		sender      *fake.FakeMetricSender
		clientPool  *mockClientPool
		client      *mockClient
		logger      *gosteno.Logger
		forwarder   *dopplerforwarder.DopplerForwarder
		fakeWrapper *mockNetworkWrapper
		message     []byte
	)

	BeforeEach(func() {
		message = []byte("I am a message!")

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		client = newMockClient()
		clientPool = newMockClientPool()
		clientPool.RandomClientOutput.client <- client
		close(clientPool.RandomClientOutput.err)

		logger = loggertesthelper.Logger()
		loggertesthelper.TestLoggerSink.Clear()

		fakeWrapper = newMockNetworkWrapper()
	})

	JustBeforeEach(func() {
		forwarder = dopplerforwarder.New(fakeWrapper, clientPool, logger)
	})

	Context("client selection", func() {
		It("selects a random client", func() {
			close(fakeWrapper.WriteOutput.ret0)
			_, err := forwarder.Write(message)
			Expect(err).ToNot(HaveOccurred())
			Eventually(fakeWrapper.WriteInput.client).Should(Receive(Equal(client)))
			Eventually(fakeWrapper.WriteInput.message).Should(Receive(Equal(message)))
		})

		Context("when selecting a client errors", func() {
			It("logs an error and returns", func() {
				close(fakeWrapper.WriteOutput.ret0)
				clientPool.RandomClientOutput.err = make(chan error, 1)
				clientPool.RandomClientOutput.err <- errors.New("boom")
				_, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to pick a client"))
				Consistently(fakeWrapper.WriteCalled).ShouldNot(Receive())
			})
		})

		Context("when networkWrapper write fails", func() {
			It("logs an error and returns", func() {
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				_, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to write message"))
			})
		})

		Context("when it errors", func() {
			It("returns an error and a zero byte count", func() {
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				n, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Expect(n).To(Equal(0))
			})
		})
	})
})
