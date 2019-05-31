package ingress_test

import (
	"net"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/rlp/internal/ingress"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		pool *ingress.Pool
	)

	BeforeEach(func() {
		pool = ingress.NewPool(2, grpc.WithInsecure())
	})

	Describe("Register() & Close()", func() {
		var (
			listeners            []net.Listener
			lis1, lis2           net.Listener
			accepter1, accepter2 chan bool
		)

		BeforeEach(func() {
			lis1, accepter1 = accepter(startListener("127.0.0.1:0"))
			lis2, accepter2 = accepter(startListener("127.0.0.1:0"))
			listeners = append(listeners, lis1, lis2)
		})

		AfterEach(func(done Done) {
			defer close(done)

			for _, lis := range listeners {
				lis.Close()
			}
			listeners = nil
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

				// Drain the channel
				Eventually(accepter1, 5).ShouldNot(Receive())
				Consistently(accepter1).Should(HaveLen(0))
			})
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
					senderA.Send(resp)
					senderB.Send(resp)
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

func startListener(addr string) net.Listener {
	var lis net.Listener
	f := func() error {
		var err error
		lis, err = net.Listen("tcp", addr)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())

	return lis
}

func startGRPCServer(ds plumbing.DopplerServer, addr string) (net.Listener, *grpc.Server) {
	lis := startListener(addr)
	s := grpc.NewServer()
	plumbing.RegisterDopplerServer(s, ds)
	go s.Serve(lis)

	return lis, s
}
