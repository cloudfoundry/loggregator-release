package plumbing_test

import (
	"net"

	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

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
		pool = plumbing.NewPool(2, grpc.WithInsecure())
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
			lis1, accepter1 = accepter(startListener(":0"))
			lis2, accepter2 = accepter(startListener(":0"))
			listeners = append(listeners, lis1, lis2)
		})

		Describe("Register()", func() {
			It("fills pool with connections to each doppler", func() {
				pool.RegisterDoppler(lis1.Addr().String())
				pool.RegisterDoppler(lis2.Addr().String())

				Eventually(accepter1).Should(HaveLen(2))
				Eventually(accepter2).Should(HaveLen(2))
			})
		})

		Describe("Close()", func() {
			BeforeEach(func() {
				pool.RegisterDoppler(lis1.Addr().String())
			})

			It("stops the gRPC connections", func() {
				pool.Close(lis1.Addr().String())
				lis1.Close()

				Eventually(accepter1, 5).Should(HaveLen(0))
				Consistently(accepter1).Should(HaveLen(0))
			})
		})
	})

	Describe("Subscribe()", func() {
		var (
			mockDoppler1 *mockDopplerServer
			lis1         net.Listener
			server1      *grpc.Server

			ctx context.Context
			req *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			mockDoppler1 = newMockDopplerServer()

			ctx = context.Background()

			req = &plumbing.SubscriptionRequest{
				ShardID: "some-shard-id",
			}
		})

		Context("when the doppler is already available", func() {
			BeforeEach(func() {
				lis1, server1 = startGRPCServer(mockDoppler1, ":0")
				listeners = append(listeners, lis1)
				servers = append(servers, server1)

				pool.RegisterDoppler(lis1.Addr().String())
			})

			It("picks a random connection to subscribe to", func() {
				data := make(chan []byte, 100)
				for i := 0; i < 10; i++ {
					rx := fetchRx(pool, lis1.Addr().String(), ctx, req)

					go consumeReceiver(rx, data)
				}

				sender11 := captureSubscribeSender(mockDoppler1)
				sender12 := captureSubscribeSender(mockDoppler1)

				for i := 0; i < 10; i++ {
					resp := &plumbing.Response{
						Payload: []byte("some-data"),
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

	Describe("ContainerMetrics() & RecentLogs()", func() {
		var (
			mockDoppler1 *mockDopplerServer
			lis1         net.Listener
			server1      *grpc.Server

			ctx  context.Context
			cReq *plumbing.ContainerMetricsRequest
			lReq *plumbing.RecentLogsRequest
		)

		BeforeEach(func() {
			mockDoppler1 = newMockDopplerServer()

			ctx = context.Background()

			cReq = &plumbing.ContainerMetricsRequest{
				AppID: "some-app-id",
			}

			lReq = &plumbing.RecentLogsRequest{
				AppID: "some-app-id",
			}
		})

		Context("when the doppler is registered", func() {
			BeforeEach(func() {
				lis1, server1 = startGRPCServer(mockDoppler1, ":0")
				listeners = append(listeners, lis1)
				servers = append(servers, server1)

				pool.RegisterDoppler(lis1.Addr().String())
			})

			Describe("ContainerMetrics()", func() {

				var fetchResp = func(addr string, ctx context.Context, req *plumbing.ContainerMetricsRequest) [][]byte {
					var resp *plumbing.ContainerMetricsResponse
					f := func() error {
						var err error
						resp, err = pool.ContainerMetrics(addr, ctx, req)
						return err
					}
					Eventually(f).ShouldNot(HaveOccurred())
					return resp.Payload
				}

				It("returns the metrics", func() {
					payload := [][]byte{
						[]byte("some-data-1"),
						[]byte("some-data-2"),
					}
					mockDoppler1.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
						Payload: payload,
					}
					mockDoppler1.ContainerMetricsOutput.Err <- nil
					resp := fetchResp(lis1.Addr().String(), ctx, cReq)

					Expect(resp).To(Equal(payload))
				})
			})

			Describe("RecentLogs()", func() {
				var fetchResp = func(addr string, ctx context.Context, req *plumbing.RecentLogsRequest) [][]byte {
					var resp *plumbing.RecentLogsResponse
					f := func() error {
						var err error
						resp, err = pool.RecentLogs(addr, ctx, req)
						return err
					}
					Eventually(f).ShouldNot(HaveOccurred())
					return resp.Payload
				}

				It("returns the metrics", func() {
					payload := [][]byte{
						[]byte("some-data-1"),
						[]byte("some-data-2"),
					}
					mockDoppler1.RecentLogsOutput.Resp <- &plumbing.RecentLogsResponse{
						Payload: payload,
					}
					mockDoppler1.RecentLogsOutput.Err <- nil
					resp := fetchResp(lis1.Addr().String(), ctx, lReq)

					Expect(resp).To(Equal(payload))
				})
			})

		})
	})
})

func consumeReceiver(rx plumbing.Doppler_SubscribeClient, data chan []byte) {
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
	req *plumbing.SubscriptionRequest) plumbing.Doppler_SubscribeClient {

	var rx plumbing.Doppler_SubscribeClient
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
