package retrystrategy_test

import (
	"doppler/sinks/retrystrategy"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetryStrategy", func() {
	Describe("ExponentialRetryStrategy", func() {

		var backoffTests = []struct {
			backoffCount int
			expected     time.Duration
		}{
			{1, 1000},
			{2, 2000},
			{3, 4000},
			{4, 8000},
			{5, 16000},
			{11, 1024000},    //1.024s
			{12, 2048000},    //2.048s
			{20, 524288000},  //8m and a bit
			{21, 1048576000}, //17m28.576s
			{22, 2097152000}, //34m57.152s
			{23, 4194304000}, //1h9m54.304s
			{24, 4194304000}, //1h9m54.304s
			{25, 4194304000}, //1h9m54.304s
			{26, 4194304000}, //1h9m54.304s
		}

		It("backs off exponentially with different random seeds", func() {
			rand.Seed(1)
			strategy := retrystrategy.NewExponentialRetryStrategy()
			otherStrategy := retrystrategy.NewExponentialRetryStrategy()

			Expect(strategy(0).String()).To(Equal("0"))
			Expect(otherStrategy(0).String()).To(Equal("0"))

			var backoff time.Duration
			var oldBackoff time.Duration
			var otherBackoff time.Duration
			var otherOldBackoff time.Duration

			for _, bt := range backoffTests {
				delta := int(bt.expected / 10)
				for i := 0; i < 10; i++ {
					backoff = strategy(bt.backoffCount)
					otherBackoff = otherStrategy(bt.backoffCount)

					Expect(backoff).ToNot(Equal(otherBackoff))

					Expect(bt.expected.Seconds()).To(BeNumerically("~", backoff.Seconds(), delta))
					Expect(bt.expected.Seconds()).To(BeNumerically("~", otherBackoff.Seconds(), delta))

					Expect(oldBackoff).ToNot(Equal(backoff))
					Expect(otherOldBackoff).ToNot(Equal(otherBackoff))

					oldBackoff = backoff
					otherOldBackoff = otherBackoff
				}
			}
		})
	})
})
