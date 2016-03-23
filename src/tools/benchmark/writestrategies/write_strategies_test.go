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
				writeStrategy *writestrategies.ConstantWriteStrategy
			)

			BeforeEach(func() {
				writer = mockWriter{}
				writeStrategy = writestrategies.NewConstantWriteStrategy(&mockGenerator{}, &writer, 1000)
			})

			Measure("writes messages at a constant rate", func(b Benchmarker) {
				go writeStrategy.StartWriter()

				time.Sleep(time.Millisecond * 50)
				writes := atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically("~", 50, 10))

				time.Sleep(time.Millisecond * 50)
				writes = atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically("~", 100, 10))

				writeStrategy.Stop()
			}, 1)

			Measure("stops writing messages when the stopChan is closed", func(b Benchmarker) {
				go writeStrategy.StartWriter()

				time.Sleep(time.Millisecond * 50)
				writes := atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 40))
				Expect(writes).To(BeNumerically("<", 60))

				writeStrategy.Stop()

				time.Sleep(time.Millisecond * 50)
				writes = atomic.LoadUint32(&writer.count)
				Expect(writes).To(BeNumerically(">", 40))
				Expect(writes).To(BeNumerically("<", 60))
			}, 1)
		})
	})

	Context("BurstWriteStrategy", func() {
		Describe("StartWriter", func() {
			var (
				writer        mockWriter
				writeStrategy *writestrategies.BurstWriteStrategy
				params        writestrategies.BurstParameters
			)

			BeforeEach(func() {
				writer = mockWriter{}
				params = writestrategies.BurstParameters{
					Minimum:   10,
					Maximum:   100,
					Frequency: 900 * time.Millisecond,
				}

			})

			JustBeforeEach(func() {
				writeStrategy = writestrategies.NewBurstWriteStrategy(&mockGenerator{}, &writer, params)
			})

			It("writes messages in bursts", func() {
				go writeStrategy.StartWriter()
				defer writeStrategy.Stop()

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

				writeStrategy.Stop()

				numWrites := writes()
				Consistently(writes).Should(Equal(numWrites))
			})

			Context("with an equal minimum and maximum", func() {
				BeforeEach(func() {
					params.Minimum = 100
					params.Maximum = 100
				})

				It("uses a constant burst number", func() {
					go writeStrategy.StartWriter()
					defer writeStrategy.Stop()

					writes := func() uint32 {
						return atomic.LoadUint32(&writer.count)
					}
					Eventually(writes).Should(BeEquivalentTo(100))
				})
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
