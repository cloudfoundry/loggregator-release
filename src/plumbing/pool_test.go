package plumbing_test

import (
	"net"

	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		pool *plumbing.Pool

		listeners []net.Listener
		servers   []*grpc.Server
	)

	BeforeEach(func() {
		pool = plumbing.NewPool(grpc.WithTransportCredentials(insecure.NewCredentials()))
	})

	AfterEach(func(done Done) {
		defer close(done)

		for _, lis := range listeners {
			lis.Close()
		}
		listeners = nil

		for _, server := range servers {
			server.Stop()
		}
		servers = nil
	})

	Describe("Register() & Close()", func() {
		var (
			lis1, lis2           net.Listener
			accepter1, accepter2 chan bool
		)

		BeforeEach(func() {
			lis1, accepter1 = accepter(startListener("127.0.0.1:0"))
			lis2, accepter2 = accepter(startListener("127.0.0.1:0"))
			listeners = append(listeners, lis1, lis2)
		})

		Describe("Register()", func() {
			It("fills pool with connections to each doppler", func() {
				pool.RegisterDoppler(lis1.Addr().String())
				pool.RegisterDoppler(lis2.Addr().String())

				Eventually(accepter1).Should(HaveLen(1))
				Eventually(accepter2).Should(HaveLen(1))
			})
		})

		Describe("Close()", func() {
			BeforeEach(func() {
				pool.RegisterDoppler(lis1.Addr().String())
			})

			It("stops the gRPC connections", func() {
				pool.Close(lis1.Addr().String())
				lis1.Close()

				// Drain the channel
				Eventually(accepter1, 5).ShouldNot(Receive())
				Consistently(accepter1).Should(HaveLen(0))
			})
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
					sender11.Send(resp)
					sender12.Send(resp)
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

func accepter(lis net.Listener) (net.Listener, chan bool) {
	c := make(chan bool, 100)
	go func() {
		var dontGC []net.Conn
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}

			dontGC = append(dontGC, conn)
			c <- true
		}
	}()
	return lis, c
}
