package app_test

import (
	"context"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/router/app"
	"code.cloudfoundry.org/loggregator/testservers"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var (
		grpcConfig app.GRPC
		router     *app.Router
		spyAgent   *spyAgent
	)

	BeforeEach(func() {
		grpcConfig = app.GRPC{
			CAFile:   testservers.Cert("loggregator-ca.crt"),
			CertFile: testservers.Cert("doppler.crt"),
			KeyFile:  testservers.Cert("doppler.key"),
		}
		spyAgent = newSpyAgent("localhost:0")
		router = app.NewRouter(
			grpcConfig,
			app.WithMetricReporting(
				"localhost:0",
				app.Agent{
					GRPCAddress: spyAgent.addr,
				},
				100,
				"doppler",
			),
		)
		router.Start()
	})

	AfterEach(func() {
		spyAgent.stop()
		router.Stop()
	})

	Describe("Addrs()", func() {
		It("returns a struct with all the addrs", func() {
			addrs := router.Addrs()

			Expect(addrs.Health).ToNot(Equal(""))
			Expect(addrs.Health).ToNot(Equal("0.0.0.0:0"))
			Expect(addrs.GRPC).ToNot(Equal(""))
			Expect(addrs.GRPC).ToNot(Equal("0.0.0.0:0"))
		})
	})

	Describe("MetricReporting", func() {
		It("reports metrics with source ID", func() {
			Expect(spyAgent.read().GetSourceId()).To(Equal("doppler"))
		})
	})

	Describe("V2 Ingress", func() {
		var (
			ingressClient loggregator_v2.IngressClient
			egressClient  loggregator_v2.EgressClient
		)

		BeforeEach(func() {
			addrs := router.Addrs()

			ingressClient = createRouterV2IngressClient(addrs.GRPC, grpcConfig)
			egressClient = createRouterV2EgressClient(addrs.GRPC, grpcConfig)
		})

		Describe("Sender", func() {
			It("sends envelopes that can be read with an egress client", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				sender, err := ingressClient.Sender(ctx)
				Expect(err).ToNot(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					ticker := time.NewTicker(50 * time.Millisecond)
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							err := sender.Send(genericLogEnvelope())
							Expect(err).ToNot(HaveOccurred())
						}
					}
				}()

				rcvr, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())

				batch, err := rcvr.Recv()
				Expect(err).ToNot(HaveOccurred())
				Expect(batch.GetBatch()).ToNot(BeEmpty())
			})
		})

		Describe("BatchSender", func() {
			It("sends envelopes in batches that can be read with an egress client", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				sender, err := ingressClient.BatchSender(ctx)
				Expect(err).ToNot(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					ticker := time.NewTicker(50 * time.Millisecond)
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							err := sender.Send(&loggregator_v2.EnvelopeBatch{
								Batch: []*loggregator_v2.Envelope{
									genericLogEnvelope(),
									genericLogEnvelope(),
								},
							})
							Expect(err).ToNot(HaveOccurred())
						}
					}
				}()

				rcvr, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				})
				Expect(err).ToNot(HaveOccurred())

				batch, err := rcvr.Recv()
				Expect(err).ToNot(HaveOccurred())
				Expect(batch.GetBatch()).ToNot(BeEmpty())
			})
		})

		Describe("Send", func() {
			It("returns an unimplemented error", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				_, err := ingressClient.Send(ctx, &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						genericLogEnvelope(),
					},
				})
				Expect(err).To(MatchError("rpc error: code = Unimplemented desc = this endpoint is not yet implemented"))
			})
		})
	})

	Describe("Selectors", func() {
		Context("when no selectors are given", func() {
			It("should not egress any envelopes", func() {
				addrs := router.Addrs()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ingressClient := createRouterV2IngressClient(addrs.GRPC, grpcConfig)
				sender, err := ingressClient.BatchSender(ctx)
				Expect(err).ToNot(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					ticker := time.NewTicker(50 * time.Millisecond)
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							err := sender.Send(&loggregator_v2.EnvelopeBatch{
								Batch: []*loggregator_v2.Envelope{
									genericLogEnvelope(),
								},
							})
							if err != nil {
								println(err.Error())
							}
						}
					}
				}()

				client := createRouterV2EgressClient(addrs.GRPC, grpcConfig)

				rx, err := client.BatchedReceiver(context.Background(), &loggregator_v2.EgressBatchRequest{
					Selectors: nil,
				})

				Expect(err).ToNot(HaveOccurred())

				results := make(chan int, 100)

				go func() {
					batch, _ := rx.Recv()
					results <- len(batch.GetBatch())
				}()

				Consistently(results, 3).Should(BeEmpty())
			})
		})
	})
})

func genericLogEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("hello world"),
			},
		},
	}
}

func createRouterV2IngressClient(addr string, g app.GRPC) loggregator_v2.IngressClient {
	return loggregator_v2.NewIngressClient(grpcDial(addr, g))
}

func createRouterV2EgressClient(addr string, g app.GRPC) loggregator_v2.EgressClient {
	return loggregator_v2.NewEgressClient(grpcDial(addr, g))
}

func grpcDial(addr string, g app.GRPC) *grpc.ClientConn {
	creds, err := plumbing.NewClientCredentials(
		g.CertFile,
		g.KeyFile,
		g.CAFile,
		"doppler",
	)
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}

	return conn
}

type spyAgent struct {
	loggregator_v2.IngressServer
	envs chan *loggregator_v2.Envelope
	s    *grpc.Server
	addr string
}

func newSpyAgent(addr string) *spyAgent {
	creds, err := plumbing.NewServerCredentials(
		testservers.Cert("metron.crt"),
		testservers.Cert("metron.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())
	a := &spyAgent{
		envs: make(chan *loggregator_v2.Envelope, 100),
		s:    grpc.NewServer(grpc.Creds(creds)),
	}
	lis, err := net.Listen("tcp", addr)
	Expect(err).ToNot(HaveOccurred())

	a.addr = lis.Addr().String()

	loggregator_v2.RegisterIngressServer(a.s, a)
	go a.s.Serve(lis)

	return a
}

func (a *spyAgent) Sender(r loggregator_v2.Ingress_SenderServer) error {
	for {
		e, err := r.Recv()
		if err != nil {
			return nil
		}
		a.envs <- e
	}
}

func (a *spyAgent) read() *loggregator_v2.Envelope {
	return <-a.envs
}

func (a *spyAgent) stop() {
	a.s.Stop()
}
