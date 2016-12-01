package legacyclientpool_test

import (
	"errors"
	"metron/legacyclientpool"
	"testing"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
)

func TestCombinedPool(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (
		Expectation,
		[]*mockPool,
		*legacyclientpool.CombinedPool,
	) {
		mockPoolA := newMockPool()
		mockPoolB := newMockPool()
		pool := legacyclientpool.NewCombinedPool(mockPoolA, mockPoolB)

		return expect.New(t), []*mockPool{mockPoolA, mockPoolB}, pool
	})

	o.Spec("it writes to the first pool", func(
		t *testing.T,
		expect Expectation,
		mockPools []*mockPool,
		combinedPool *legacyclientpool.CombinedPool,
	) {
		close(mockPools[0].WriteOutput.Ret0)
		close(mockPools[1].WriteOutput.Ret0)
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		expect(err == nil).To.Equal(true)

		expect(mockPools[0].WriteCalled).To.Have.Len(1).Else.FailNow()
		expect(<-mockPools[0].WriteInput.Message).To.Equal(msg)
		expect(<-mockPools[0].WriteInput.Chainers).To.Equal([]metricbatcher.BatchCounterChainer{nil})
		expect(mockPools[1].WriteCalled).To.Have.Len(0)
	})

	o.Spec("it writes to the second pool when the first fails", func(
		t *testing.T,
		expect Expectation,
		mockPools []*mockPool,
		combinedPool *legacyclientpool.CombinedPool,
	) {
		mockPools[0].WriteOutput.Ret0 <- errors.New("some-error")
		close(mockPools[1].WriteOutput.Ret0)
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		expect(err == nil).To.Equal(true)

		expect(mockPools[1].WriteCalled).To.Have.Len(1).Else.FailNow()
		expect(<-mockPools[1].WriteInput.Message).To.Equal(msg)
		expect(<-mockPools[1].WriteInput.Chainers).To.Equal([]metricbatcher.BatchCounterChainer{nil})
	})

	o.Spec("returns an error when all the pools fail", func(
		t *testing.T,
		expect Expectation,
		mockPools []*mockPool,
		combinedPool *legacyclientpool.CombinedPool,
	) {
		mockPools[0].WriteOutput.Ret0 <- errors.New("some-error")
		mockPools[1].WriteOutput.Ret0 <- errors.New("some-error")
		msg := []byte("a message")

		err := combinedPool.Write(msg, nil)
		expect(err == nil).To.Equal(false)
	})
}
