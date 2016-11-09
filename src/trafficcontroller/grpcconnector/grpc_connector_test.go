package grpcconnector_test

import (
	"fmt"
	"net"
	"plumbing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/dopplerservice"
	"trafficcontroller/grpcconnector"
)

var _ = Describe("GRPCConnector", func() {
	var (
		req         *plumbing.SubscriptionRequest
		connector   *grpcconnector.GRPCConnector
		listeners   []net.Listener
		grpcServers []*grpc.Server

		mockDopplerServerA *mockDopplerServer
		mockDopplerServerB *mockDopplerServer
		mockFinder         *mockFinder

		mockBatcher *mockMetaMetricBatcher
		mockChainer *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockDopplerServerA = newMockDopplerServer()
		mockDopplerServerB = newMockDopplerServer()
		mockFinder = newMockFinder()

		tlsConfig, err := plumbing.NewTLSConfig(
			"./fixtures/client.crt",
			"./fixtures/client.key",
			"./fixtures/loggregator-ca.crt",
			"doppler",
		)
		Expect(err).ToNot(HaveOccurred())

		pool := grpcconnector.NewPool(2, tlsConfig)

		mockBatcher = newMockMetaMetricBatcher()
		mockChainer = newMockBatchCounterChainer()

		lisA, serverA := startGRPCServer(mockDopplerServerA, ":0")
		lisB, serverB := startGRPCServer(mockDopplerServerB, ":0")
		listeners = append(listeners, lisA, lisB)
		grpcServers = append(grpcServers, serverA, serverB)

		req = &plumbing.SubscriptionRequest{
			ShardID: "test-sub-id",
			Filter: &plumbing.Filter{
				AppID: "test-app-id",
			},
		}

		connector = grpcconnector.New(5, pool, mockFinder, mockBatcher)

		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)
	})

	AfterEach(func() {
		for _, lis := range listeners {
			lis.Close()
		}

		for _, server := range grpcServers {
			server.Stop()
		}

		listeners = nil
		grpcServers = nil
	})

	Describe("Subscribe()", func() {
		var (
			ctx       context.Context
			cancelCtx func()
		)

		BeforeEach(func() {
			ctx, cancelCtx = context.WithCancel(context.Background())
		})

		Context("when no dopplers are available", func() {
			Context("when a doppler comes online after stream is established", func() {
				var (
					data <-chan []byte
					errs <-chan error
				)

				BeforeEach(func() {
					event := dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					var ready chan struct{}
					data, errs, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())

					mockFinder.NextOutput.Ret0 <- event
				})

				It("connects to the doppler with the correct request", func() {
					Eventually(mockDopplerServerA.SubscribeInput).Should(
						BeCalled(With(req, Not(BeNil()))),
					)
				})

				It("returns data from both dopplers", func() {
					senderA := captureSubscribeSender(mockDopplerServerA)
					senderB := captureSubscribeSender(mockDopplerServerB)

					senderA.Send(&plumbing.Response{
						Payload: []byte("some-data-a"),
					})
					Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))

					senderB.Send(&plumbing.Response{
						Payload: []byte("some-data-b"),
					})
					Eventually(data).Should(Receive(Equal([]byte("some-data-b"))))
				})

				It("increments a batch count", func() {
					senderA := captureSubscribeSender(mockDopplerServerA)

					senderA.Send(&plumbing.Response{
						Payload: []byte("some-data-a"),
					})

					Eventually(mockChainer.IncrementCalled).Should(BeCalled())
				})

				It("does not close the doppler connection when a client exits", func() {
					Eventually(mockDopplerServerA.SubscribeInput.Stream).Should(Receive())
					cancelCtx()

					newData, _, ready := readFromSubscription(context.Background(), req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.SubscribeCalled).Should(HaveLen(2))

					senderA := captureSubscribeSender(mockDopplerServerA)
					senderA.Send(&plumbing.Response{
						Payload: []byte("some-data-a"),
					})

					Eventually(newData, 5).Should(Receive())
				})

				Context("when another doppler comes online", func() {
					var secondEvent dopplerservice.Event

					BeforeEach(func() {
						Eventually(mockDopplerServerA.SubscribeCalled).Should(HaveLen(1))
						Eventually(mockDopplerServerB.SubscribeCalled).Should(HaveLen(1))

						mockDopplerServerC := newMockDopplerServer()
						lisC, serverC := startGRPCServer(mockDopplerServerC, ":0")
						listeners = append(listeners, lisC)
						grpcServers = append(grpcServers, serverC)

						secondEvent = dopplerservice.Event{
							GRPCDopplers: createGrpcURIs(listeners),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent
					})

					It("does not reconnect to pre-existing dopplers", func() {
						Consistently(mockDopplerServerA.SubscribeCalled).Should(HaveLen(1))
						Consistently(mockDopplerServerB.SubscribeCalled).Should(HaveLen(1))
					})
				})

				Context("when a doppler is removed permanently", func() {
					var secondEvent dopplerservice.Event

					BeforeEach(func() {
						senderA := captureSubscribeSender(mockDopplerServerA)
						senderA.Send(&plumbing.Response{
							Payload: []byte("some-data-a"),
						})
						Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))

						secondEvent = dopplerservice.Event{
							GRPCDopplers: createGrpcURIs(listeners[1:]),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent

						listeners[0].Close()
						grpcServers[0].Stop()
						//listeners[0], grpcServers[0] = startGRPCServer(mockDopplerServerA, listeners[0].Addr().String())
						time.Sleep(1 * time.Second)
						listeners[0] = startListener(listeners[0].Addr().String())
					})

					It("does not attempt reconnect", func() {
						c := make(chan net.Conn, 100)
						go func(lis net.Listener) {
							conn, _ := lis.Accept()
							c <- conn
						}(listeners[0])
						Consistently(c, 2).ShouldNot(Receive())
					})
				})

				Context("when the consumer disconnects", func() {
					BeforeEach(func() {
						cancelCtx()
					})

					It("returns an error", func() {
						Eventually(errs).Should(Receive())
					})
				})

				Context("when finder service falsely reports doppler going away", func() {
					var (
						secondEvent dopplerservice.Event
						senderA     plumbing.Doppler_SubscribeServer
					)
					BeforeEach(func() {
						senderA = captureSubscribeSender(mockDopplerServerA)

						secondEvent = dopplerservice.Event{
							GRPCDopplers: createGrpcURIs(nil),
						}

						mockFinder.NextOutput.Ret0 <- secondEvent
					})

					It("continues reading from the doppler", func() {
						senderA.Send(&plumbing.Response{
							Payload: []byte("some-data-a"),
						})
						Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))
					})

					It("accepts new streams", func() {
						data, _, ready := readFromSubscription(ctx, req, connector)
						Eventually(ready).Should(BeClosed())
						sender := captureSubscribeSender(mockDopplerServerA)
						sender.Send(&plumbing.Response{
							Payload: []byte("some-data"),
						})
						Eventually(data).Should(Receive())
					})

					Context("finder event readvertises the doppler", func() {
						var (
							thirdEvent dopplerservice.Event
						)

						BeforeEach(func() {
							thirdEvent = dopplerservice.Event{
								GRPCDopplers: createGrpcURIs(listeners),
							}

							mockFinder.NextOutput.Ret0 <- thirdEvent
						})

						It("doesn't create a second connection", func() {
							Consistently(mockDopplerServerA.SubscribeCalled, 1).Should(HaveLen(1))
						})

						Context("when the first connection is closed", func() {
							var (
								newCtx       context.Context
								newCancelCtx func()
							)

							BeforeEach(func() {
								cancelCtx()

								newCtx, newCancelCtx = context.WithCancel(context.Background())
							})

							AfterEach(func() {
								newCancelCtx()
							})

							It("allows new connections", func() {
								// TODO: Once we're using a mock pool, use that
								// to show when the event has been processed.
								time.Sleep(time.Second)

								data, _, ready := readFromSubscription(newCtx, req, connector)
								Eventually(ready).Should(BeClosed())
								sender := captureSubscribeSender(mockDopplerServerA)
								sender.Send(&plumbing.Response{
									Payload: []byte("some-data"),
								})
								Eventually(data).Should(Receive())
							})
						})
					})
				})
			})

			Context("when a doppler comes online before stream has established", func() {
				var event dopplerservice.Event

				BeforeEach(func() {
					event = dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					mockFinder.NextOutput.Ret0 <- event
					Eventually(mockFinder.NextCalled).Should(HaveLen(2))

					_, _, ready := readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
				})

				It("connects to the doppler with the correct request", func() {
					Eventually(mockDopplerServerA.SubscribeInput).Should(
						BeCalled(With(req, Not(BeNil()))),
					)
				})
			})

			Context("when a consumer is too slow", func() {
				var event dopplerservice.Event

				BeforeEach(func() {
					event = dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					mockFinder.NextOutput.Ret0 <- event
					Eventually(mockFinder.NextCalled).Should(HaveLen(2))
				})

				It("returns an error", func() {
					r, _ := connector.Subscribe(ctx, req)
					senderA := captureSubscribeSender(mockDopplerServerA)
					for i := 0; i < 50; i++ {
						senderA.Send(&plumbing.Response{
							Payload: []byte(fmt.Sprintf("some-data-a-%d", i)),
						})
					}

					time.Sleep(2 * time.Second)

					f := func() error {
						_, err := r.Recv()
						return err
					}
					Eventually(f).Should(Not(BeNil()))
					Eventually(mockBatcher.BatchAddCounterInput).Should(
						BeCalled(With(
							"grpcConnector.slowConsumers",
							BeNumerically("==", 1),
						)),
					)
				})
			})

			Context("when new doppler is not available right away", func() {
				var (
					event dopplerservice.Event
					data  <-chan []byte
					errs  <-chan error
				)

				BeforeEach(func() {
					event = dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					listeners[0].Close()
					grpcServers[0].Stop()

					mockFinder.NextOutput.Ret0 <- event

					data, errs, _ = readFromSubscription(ctx, req, connector)
					Eventually(mockDopplerServerB.SubscribeCalled).ShouldNot(HaveLen(0))
					listeners[0], grpcServers[0] = startGRPCServer(mockDopplerServerA, listeners[0].Addr().String())
				})

				It("attempts to reconnect", func() {
					Eventually(mockDopplerServerA.SubscribeInput.Stream, 3).Should(Receive())
				})
			})

			Context("when an active doppler closes after reading", func() {
				var (
					event   dopplerservice.Event
					data    <-chan []byte
					errs    <-chan error
					ready   <-chan struct{}
					senderA plumbing.Doppler_SubscribeServer
				)

				BeforeEach(func() {
					event = dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					mockFinder.NextOutput.Ret0 <- event

					data, errs, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.SubscribeCalled).Should(HaveLen(1))
					Eventually(mockDopplerServerB.SubscribeCalled).Should(HaveLen(1))

					senderA = captureSubscribeSender(mockDopplerServerA)
					senderA.Send(&plumbing.Response{
						Payload: []byte("test-payload-1"),
					})

					Eventually(data).Should(Receive(Equal([]byte("test-payload-1"))))

					listeners[0].Close()
					grpcServers[0].Stop()
					listeners[0], grpcServers[0] = startGRPCServer(mockDopplerServerA, listeners[0].Addr().String())
				})

				It("attempts to reconnect", func() {
					senderA = captureSubscribeSender(mockDopplerServerA)
					senderA.Send(&plumbing.Response{
						Payload: []byte("test-payload-2"),
					})

					Eventually(data, 5).Should(Receive(Equal([]byte("test-payload-2"))))
				})
			})
		})
	})

	Describe("ContainerMetrics() and RecentLogs()", func() {
		var (
			ctx       context.Context
			cancelCtx func()
		)

		BeforeEach(func() {
			ctx, cancelCtx = context.WithCancel(context.Background())
		})

		Context("with doppler connections established", func() {
			var (
				testMetricA    = []byte("test-container-metric-a")
				testMetricB    = []byte("test-container-metric-b")
				testRecentLogA = []byte("test-recent-log-a")
				testRecentLogB = []byte("test-recent-log-b")
			)

			BeforeEach(func() {
				event := dopplerservice.Event{
					GRPCDopplers: createGrpcURIs(listeners),
				}
				mockFinder.NextOutput.Ret0 <- event

				mockDopplerServerA.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
					Payload: [][]byte{testMetricA},
				}
				mockDopplerServerA.ContainerMetricsOutput.Err <- nil
				mockDopplerServerB.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
					Payload: [][]byte{testMetricB},
				}
				mockDopplerServerB.ContainerMetricsOutput.Err <- nil

				mockDopplerServerA.RecentLogsOutput.Resp <- &plumbing.RecentLogsResponse{
					Payload: [][]byte{testRecentLogA},
				}
				mockDopplerServerA.RecentLogsOutput.Err <- nil
				mockDopplerServerB.RecentLogsOutput.Resp <- &plumbing.RecentLogsResponse{
					Payload: [][]byte{testRecentLogB},
				}
				mockDopplerServerB.RecentLogsOutput.Err <- nil
			})

			It("can request container metrics", func() {
				f := func() [][]byte {
					return connector.ContainerMetrics(ctx, "test-app-id")
				}
				Eventually(f).Should(ConsistOf(testMetricA, testMetricB))
			})

			It("can request recent logs", func() {
				f := func() [][]byte {
					return connector.RecentLogs(ctx, "test-app-id")
				}
				Eventually(f).Should(ConsistOf(testRecentLogA, testRecentLogB))
			})
		})
	})
})

func readFromSubscription(ctx context.Context, req *plumbing.SubscriptionRequest, connector *grpcconnector.GRPCConnector) (<-chan []byte, <-chan error, chan struct{}) {
	data := make(chan []byte, 100)
	errs := make(chan error, 100)
	ready := make(chan struct{})

	go func() {
		defer GinkgoRecover()
		r, err := connector.Subscribe(ctx, req)
		close(ready)
		Expect(err).ToNot(HaveOccurred())
		for {
			d, e := r.Recv()
			data <- d
			errs <- e
		}
	}()

	return data, errs, ready
}

func createGrpcURIs(listeners []net.Listener) []string {
	var results []string
	for _, lis := range listeners {
		results = append(results, lis.Addr().String())
	}
	return results
}

func captureSubscribeSender(doppler *mockDopplerServer) plumbing.Doppler_SubscribeServer {
	var server plumbing.Doppler_SubscribeServer
	EventuallyWithOffset(1, doppler.SubscribeInput.Stream, 5).Should(Receive(&server))
	return server
}
