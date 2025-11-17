package plumbing_test

import (
	"context"

	"code.cloudfoundry.org/loggregator-release/src/plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		pool *plumbing.Pool
	)

	BeforeEach(func() {
		pool = plumbing.NewPool(grpc.WithTransportCredentials(insecure.NewCredentials()))
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

			ctx context.Context
			req *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			mockDoppler1 = startMockDopplerServer()

			ctx = context.Background()

			req = &plumbing.SubscriptionRequest{
				ShardID: "some-shard-id",
			}
		})

		AfterEach(func() {
			mockDoppler1.Stop()
		})

		Context("when the doppler is already available", func() {
			BeforeEach(func() {
				pool.RegisterDoppler(mockDoppler1.addr.String())
			})

			It("picks a random connection to subscribe to", func() {
				data := make(chan [][]byte, 100)
				for i := 0; i < 10; i++ {
					rx := fetchRx(pool, mockDoppler1.addr.String(), ctx, req)

					go consumeReceiver(rx, data)
				}

				sender11 := captureSubscribeSender(mockDoppler1)
				sender12 := captureSubscribeSender(mockDoppler1)

				for i := 0; i < 10; i++ {
					resp := &plumbing.BatchResponse{
						Payload: [][]byte{[]byte("some-data")},
					}
					err11 := sender11.Send(resp)
					err12 := sender12.Send(resp)

					Expect(err11).NotTo(HaveOccurred())
					Expect(err12).NotTo(HaveOccurred())
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

func consumeReceiver(rx plumbing.Doppler_BatchSubscribeClient, data chan [][]byte) {
	for {
		d, err := rx.Recv()
		if err != nil {
			return
		}
		data <- d.Payload
	}
}

func fetchRx(
	pool *plumbing.Pool,
	addr string,
	ctx context.Context,
	req *plumbing.SubscriptionRequest) plumbing.Doppler_BatchSubscribeClient {

	var rx plumbing.Doppler_BatchSubscribeClient
	f := func() error {
		var err error
		rx, err = pool.Subscribe(addr, ctx, req)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())
	return rx
}
