package ingress_test

import (
	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/rlp/internal/ingress"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		pool *ingress.Pool
	)

	BeforeEach(func() {
		pool = ingress.NewPool(2, grpc.WithTransportCredentials(insecure.NewCredentials()))
	})

	Describe("Register()", func() {
		BeforeEach(func() {
			pool.RegisterDoppler("192.0.2.10:8080")
			pool.RegisterDoppler("192.0.2.11:8080")
		})

		It("adds entries to the pool", func() {
			Eventually(pool.Size).Should(Equal(2))
		})
	})

	Describe("Close()", func() {
		BeforeEach(func() {
			pool.RegisterDoppler("192.0.2.10:8080")
			pool.RegisterDoppler("192.0.2.11:8080")
		})

		It("removes entries from the pool", func() {
			Eventually(pool.Size).Should(Equal(2))
			pool.Close("192.0.2.11:8080")
			Eventually(pool.Size).Should(Equal(1))
		})
	})

	Describe("Subscribe()", func() {
		var (
			mockDoppler1 *spyRouter
			ctx          context.Context
			req          *loggregator_v2.EgressBatchRequest
		)

		BeforeEach(func() {
			mockDoppler1 = startMockDopplerServer()

			ctx = context.Background()

			req = &loggregator_v2.EgressBatchRequest{
				ShardId: "some-shard-id",
			}
		})

		Context("when the doppler is already available", func() {
			BeforeEach(func() {
				pool.RegisterDoppler(mockDoppler1.addr.String())
			})

			It("picks a random connection to subscribe to", func() {
				data := make(chan []*loggregator_v2.Envelope, 100)
				for i := 0; i < 10; i++ {
					rx := fetchRx(pool, mockDoppler1.addr.String(), ctx, req)

					go consumeReceiver(rx, data)
				}

				senderA := captureSubscribeSender(mockDoppler1)
				senderB := captureSubscribeSender(mockDoppler1)

				for i := 0; i < 10; i++ {
					resp := &loggregator_v2.EnvelopeBatch{
						Batch: []*loggregator_v2.Envelope{{SourceId: "A"}},
					}
					errA := senderA.Send(resp)
					errB := senderB.Send(resp)

					Expect(errA).NotTo(HaveOccurred())
					Expect(errB).NotTo(HaveOccurred())
				}

				Eventually(data).Should(HaveLen(20))
			})
		})

		Context("when the doppler is not registered", func() {
			It("returns an error", func() {
				_, err := pool.Subscribe("invalid", ctx, req)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the doppler is registered but not up", func() {
			BeforeEach(func() {
				pool.RegisterDoppler("some-addr")
			})

			It("returns an error", func() {
				_, err := pool.Subscribe("some-addr", ctx, req)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func consumeReceiver(rx loggregator_v2.Egress_BatchedReceiverClient, data chan []*loggregator_v2.Envelope) {
	for {
		d, err := rx.Recv()
		if err != nil {
			return
		}
		data <- d.Batch
	}
}

func fetchRx(
	pool *ingress.Pool,
	addr string,
	ctx context.Context,
	req *loggregator_v2.EgressBatchRequest,
) loggregator_v2.Egress_BatchedReceiverClient {

	var rx loggregator_v2.Egress_BatchedReceiverClient
	f := func() error {
		var err error
		rx, err = pool.Subscribe(addr, ctx, req)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())
	return rx
}
