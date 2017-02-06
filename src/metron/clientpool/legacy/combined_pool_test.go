package legacy_test

import (
	"errors"
	"metron/clientpool/legacy"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Combined Pool", func() {
	var (
		mockPools    []*mockPool
		combinedPool *legacy.CombinedPool
	)
	BeforeEach(func() {
		mockPoolA := newMockPool()
		mockPoolB := newMockPool()
		combinedPool = legacy.NewCombinedPool(mockPoolA, mockPoolB)
		mockPools = []*mockPool{mockPoolA, mockPoolB}
	})

	It("it writes to the first pool", func() {
		close(mockPools[0].WriteOutput.Ret0)
		close(mockPools[1].WriteOutput.Ret0)
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(mockPools[0].WriteCalled).To(HaveLen(1))
		Expect(<-mockPools[0].WriteInput.Message).To(Equal(msg))
		Expect(<-mockPools[0].WriteInput.Chainers).To(Equal([]metricbatcher.BatchCounterChainer{nil}))
		Expect(mockPools[1].WriteCalled).To(HaveLen(0))
	})

	It("it writes to the second pool when the first fails", func() {
		mockPools[0].WriteOutput.Ret0 <- errors.New("some-error")
		close(mockPools[1].WriteOutput.Ret0)
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(mockPools[1].WriteCalled).To(HaveLen(1))
		Expect(<-mockPools[1].WriteInput.Message).To(Equal(msg))
		Expect(<-mockPools[1].WriteInput.Chainers).To(Equal([]metricbatcher.BatchCounterChainer{nil}))
	})

	It("returns an error when all the pools fail", func() {
		mockPools[0].WriteOutput.Ret0 <- errors.New("some-error")
		mockPools[1].WriteOutput.Ret0 <- errors.New("some-error")
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		Expect(err).To(HaveOccurred())
	})
})
