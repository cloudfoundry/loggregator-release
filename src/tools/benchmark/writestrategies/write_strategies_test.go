package writestrategies_test

import (
	"sync/atomic"
	"time"

	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WriteStrategies", func() {
	Context("ConstantWriteStrategy", func() {
		Describe("StartWriter", func() {
			var (
				writer        mockWriter
				stopChan      chan struct{}
				writeStrategy *writestrategies.ConstantWriteStrategy
			)

			BeforeEach(func() {
				writer = mockWriter{}
				stopChan = make(chan struct{})
				writeStrategy = writestrategies.NewConstantWriteStrategy(&mockGenerator{}, &writer, 1000, stopChan)
			})

			It("writes messages at a constant rate", func() {
				go writeStrategy.StartWriter()

				time.Sleep(time.Millisecond * 50)
				writes := atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 40))
				Expect(writes).To(BeNumerically("<", 60))

				time.Sleep(time.Millisecond * 50)
				writes = atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 90))
				Expect(writes).To(BeNumerically("<", 110))

				close(stopChan)
			})

			It("stops writing messages when the stopChan is closed", func() {
				go writeStrategy.StartWriter()

				time.Sleep(time.Millisecond * 50)
				writes := atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 40))
				Expect(writes).To(BeNumerically("<", 60))

				close(stopChan)

				time.Sleep(time.Millisecond * 50)
				writes = atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 40))
				Expect(writes).To(BeNumerically("<", 60))
			})
		})
	})

	Context("BurstWriteStrategy", func() {
		Describe("StartWriter", func() {
			var (
				writer        mockWriter
				stopChan      chan struct{}
				writeStrategy *writestrategies.BurstWriteStrategy
				params        writestrategies.BurstParameters
			)

			BeforeEach(func() {
				writer = mockWriter{}
				stopChan = make(chan struct{})
				params = writestrategies.BurstParameters{
					Minimum:   10,
					Maximum:   100,
					Frequency: 900 * time.Millisecond,
				}
				writeStrategy = writestrategies.NewBurstWriteStrategy(&mockGenerator{}, &writer, params, stopChan)
			})

			It("writes messages in bursts", func() {
				go writeStrategy.StartWriter()
				defer close(stopChan)

				writes := func() uint32 {
					return atomic.LoadUint32(&writer.count)
				}
				Eventually(writes).Should(BeNumerically(">=", params.Minimum))
				Eventually(writes).Should(BeNumerically("<=", params.Maximum))
			})

			It("stops writing after the stoChan is closed", func() {
				go writeStrategy.StartWriter()

				writes := func() uint32 {
					return atomic.LoadUint32(&writer.count)
				}
				Eventually(writes).Should(BeNumerically(">=", params.Minimum))
				Eventually(writes).Should(BeNumerically("<=", params.Maximum))

				close(stopChan)

				numWrites := writes()
				Consistently(writes).Should(Equal(numWrites))
			})
		})
	})
})

type mockWriter struct {
	count uint32
}

func (m *mockWriter) Write([]byte) {
	atomic.AddUint32(&m.count, 1)
}

type mockGenerator struct{}

func (m *mockGenerator) Generate() []byte {
	return []byte{}
}
