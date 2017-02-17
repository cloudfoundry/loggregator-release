package metric_test

import (
	"fmt"
	"math/rand"
	"metric"
	"net"
	"time"

	"google.golang.org/grpc"

	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metric", func() {

	var (
		mockConsumer *mockIngressServer
		receiver     <-chan *v2.Envelope
	)

	BeforeSuite(func() {
		var addr string
		addr, mockConsumer = startConsumer()
		metric.Setup(
			metric.WithAddr(addr),
			metric.WithSourceUUID("some-uuid"),
			metric.WithBatchInterval(250*time.Millisecond),
			metric.WithPrefix("loggregator"),
			metric.WithOrigin("metron"),
			metric.WithDeploymentMeta("some-deployment", "some-job", "some-index"),
		)

		// Seed the data
		metric.IncCounter("seed-data")

		rx := fetchReceiver(mockConsumer)
		receiver = rxToCh(rx)
	})

	Context("when a consumer is not available", func() {
		It("does not block when writing to it", func() {
			done := make(chan struct{})
			go func() {
				defer close(done)
				metric.IncCounter("some-name")
			}()

			Eventually(done).Should(BeClosed())
		})
	})

	Context("when a consumer is available", func() {
		var (
			randName string
		)

		BeforeEach(func() {
			randName = generateRandName()
		})

		Describe("IncCounter()", func() {
			It("writes a counter event periodically to the consumer", func() {
				for i := 0; i < 5; i++ {
					metric.IncCounter(randName)
				}

				var e *v2.Envelope
				f := func() bool {
					Eventually(receiver).Should(Receive(&e))

					counter := e.GetCounter()
					if counter == nil {
						return false
					}

					return counter.Name == "loggregator."+randName
				}

				Eventually(f).Should(BeTrue())
				Expect(e.Timestamp).ToNot(Equal(int64(0)))
				Expect(e.SourceId).To(Equal("some-uuid"))
				Expect(e.GetCounter().GetDelta()).To(Equal(uint64(5)))

				value, ok := e.GetTags()["origin"]
				Expect(ok).To(Equal(true))
				Expect(value.GetText()).To(Equal("metron"))
			})

			It("increments by the given value", func() {
				metric.IncCounter(randName, metric.WithIncrement(42))

				var e *v2.Envelope
				f := func() bool {
					Eventually(receiver).Should(Receive(&e))

					counter := e.GetCounter()
					if counter == nil {
						return false
					}

					return counter.Name == "loggregator."+randName
				}

				Eventually(f).Should(BeTrue())
				Expect(e.GetCounter().GetDelta()).To(Equal(uint64(42)))
			})

			It("tags with the given version", func() {
				metric.IncCounter(randName, metric.WithVersion(1, 2))
				var e *v2.Envelope
				f := func() bool {
					Eventually(receiver).Should(Receive(&e))

					counter := e.GetCounter()
					if counter == nil {
						return false
					}

					return counter.Name == "loggregator."+randName
				}
				Eventually(f).Should(BeTrue())
				Expect(e.GetTags()["metric_version"].GetText()).To(Equal("1.2"))
			})

			It("tags with meta deployment tags", func() {
				metric.IncCounter(randName, metric.WithIncrement(42))
				var e *v2.Envelope
				f := func() bool {
					Eventually(receiver).Should(Receive(&e))

					counter := e.GetCounter()
					if counter == nil {
						return false
					}

					return counter.Name == "loggregator."+randName
				}

				Eventually(f).Should(BeTrue())
				Expect(e.Tags["deployment"].GetText()).To(Equal("some-deployment"))
				Expect(e.Tags["job"].GetText()).To(Equal("some-job"))
				Expect(e.Tags["index"].GetText()).To(Equal("some-index"))
			})
		})
	})
})

func startConsumer() (string, *mockIngressServer) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	mockIngressServer := newMockIngressServer()

	s := grpc.NewServer()
	v2.RegisterIngressServer(s, mockIngressServer)
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	return lis.Addr().String(), mockIngressServer
}

func rxToCh(rx v2.Ingress_SenderServer) <-chan *v2.Envelope {
	c := make(chan *v2.Envelope, 100)
	go func() {
		for {
			e, err := rx.Recv()
			if err != nil {
				continue
			}
			c <- e
		}
	}()
	return c
}

func fetchReceiver(mockConsumer *mockIngressServer) (rx v2.Ingress_SenderServer) {
	Eventually(mockConsumer.SenderInput.Arg0, 3).Should(Receive(&rx))
	return rx
}

func generateRandName() string {
	return fmt.Sprintf("rand-name-%d", rand.Int63())
}
