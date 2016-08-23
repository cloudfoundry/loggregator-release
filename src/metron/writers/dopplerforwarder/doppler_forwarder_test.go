package dopplerforwarder_test

import (
	"errors"
	"metron/writers/dopplerforwarder"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var _ = Describe("DopplerForwarder", func() {
	var (
		clientPool  *mockClientPool
		client      *mockClient
		logger      *gosteno.Logger
		forwarder   *dopplerforwarder.DopplerForwarder
		fakeWrapper *mockNetworkWrapper
		message     []byte
	)

	BeforeEach(func() {
		message = []byte("I am a message!")

		client = newMockClient()
		clientPool = newMockClientPool()
		clientPool.RandomClientOutput.Client <- client
		close(clientPool.RandomClientOutput.Err)

		logger = loggertesthelper.Logger()
		loggertesthelper.TestLoggerSink.Clear()

		fakeWrapper = newMockNetworkWrapper()
	})

	JustBeforeEach(func() {
		forwarder = dopplerforwarder.New(fakeWrapper, clientPool, logger)
	})

	Context("client selection", func() {
		It("selects a random client", func() {
			close(fakeWrapper.WriteOutput.Ret0)
			_, err := forwarder.Write(message)
			Expect(err).ToNot(HaveOccurred())
			Eventually(fakeWrapper.WriteInput.Client).Should(Receive(Equal(client)))
			Eventually(fakeWrapper.WriteInput.Message).Should(Receive(Equal(message)))
		})

		It("passes any chainers to the wrapper", func() {
			chainers := []metricbatcher.BatchCounterChainer{
				newMockBatchCounterChainer(),
				newMockBatchCounterChainer(),
			}
			close(fakeWrapper.WriteOutput.Ret0)
			_, err := forwarder.Write(message, chainers...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(fakeWrapper.WriteInput).Should(BeCalled(
				With(client, message, chainers),
			))
		})

		Context("when selecting a client errors", func() {
			It("logs an error and returns", func() {
				close(fakeWrapper.WriteOutput.Ret0)
				clientPool.RandomClientOutput.Err = make(chan error, 1)
				clientPool.RandomClientOutput.Err <- errors.New("boom")
				_, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to pick a client"))
				Consistently(fakeWrapper.WriteCalled).ShouldNot(Receive())
			})
		})

		Context("when networkWrapper write fails", func() {
			It("logs an error and returns", func() {
				fakeWrapper.WriteOutput.Ret0 <- errors.New("boom")
				_, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("failed to write message"))
			})
		})

		Context("when it errors", func() {
			It("returns an error and a zero byte count", func() {
				fakeWrapper.WriteOutput.Ret0 <- errors.New("boom")
				n, err := forwarder.Write(message)
				Expect(err).To(HaveOccurred())
				Expect(n).To(Equal(0))
			})
		})

		Context("when it is already writing", func() {
			It("returns an error rather than blocking", func() {
				go forwarder.Write(message)
				Eventually(fakeWrapper.WriteCalled).Should(BeCalled())

				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					_, err = forwarder.Write(message)
				}()
				Eventually(done).Should(BeClosed())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("DopplerForwarder: Write already in use"))
				fakeWrapper.WriteOutput.Ret0 <- nil
			})
		})
	})
})
