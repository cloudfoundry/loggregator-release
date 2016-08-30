package backoff_test

import (
	"doppler/sinks/retrystrategy"
	"metron/backoff"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backoff", func() {
	Context("connects successfully", func() {
		It("does not return error", func() {
			fakeLogger := loggertesthelper.Logger()
			mockAdapter := backoff.NewMockAdapter()

			retryStrategy := retrystrategy.Exponential()

			err := backoff.Connect(mockAdapter, retryStrategy, fakeLogger, 3)
			Expect(err).ToNot(HaveOccurred())

		})
	})

	Context("unsuccessfully connects", func() {
		It("connects eventually using backoff strategy", func() {
			fakeLogger := loggertesthelper.Logger()
			mockAdapter := backoff.NewMockAdapter()

			mockAdapter.ConnectErr("Etcd connection error")
			retryStrategy := retrystrategy.Exponential()

			go func() {
				ticker := time.NewTicker(time.Second * 1)
				<-ticker.C
				mockAdapter.Reset()
			}()

			err := backoff.Connect(mockAdapter, retryStrategy, fakeLogger, 12)
			Expect(err).ToNot(HaveOccurred())
		})

		It("retries only the max number of retries", func() {
			fakeLogger := loggertesthelper.Logger()
			mockAdapter := backoff.NewMockAdapter()
			mockAdapter.ConnectErr("Etcd connection error")

			retryStrategy := retrystrategy.Exponential()

			err := backoff.Connect(mockAdapter, retryStrategy, fakeLogger, 3)
			Expect(err.Error()).To(ContainSubstring("3 tries"))
		})
	})
})
