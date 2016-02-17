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
		mockRetrier *mockRetrier
		retrier     dopplerforwarder.Retrier
		message     []byte
	)

	BeforeEach(func() {
		mockRetrier = nil
		retrier = nil

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
		if mockRetrier != nil {
			retrier = mockRetrier
		}
		forwarder = dopplerforwarder.New(fakeWrapper, clientPool, retrier, logger)
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

		Context("when it errors with a nil retrier", func() {
			It("returns and error, and a zero byte count", func() {
				fakeWrapper.WriteOutput.ret0 <- errors.New("boom")
				n, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Expect(n).To(Equal(0))
			})
		})

		Context("metrics", func() {
			It("emits the sentMessages metric", func() {
				close(fakeWrapper.WriteOutput.ret0)
				_, err := forwarder.Write(message)
				Expect(err).NotTo(HaveOccurred())

				Eventually(fakeWrapper.WriteInput.message).Should(Receive(Equal(message)))
				Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			})
		})
	})

	Describe("Weight", func() {
		BeforeEach(func() {
			close(fakeWrapper.WriteOutput.ret0)
			clientPool = newMockClientPool()
			clientPool.SizeOutput.ret0 <- 10
		})
		It("returns the size of the client pool", func() {
			Expect(forwarder.Weight()).To(Equal(10))
		})
	})
})
