package plumbing_test

import (
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/apoydence/eachers/testhelpers"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCConnector", func() {
	var (
		req         *plumbing.SubscriptionRequest
		connector   *plumbing.GRPCConnector
		listeners   []net.Listener
		grpcServers []*grpc.Server

		mockDopplerServerA *mockDopplerServer
		mockDopplerServerB *mockDopplerServer
		mockFinder         *mockFinder

		mockBatcher  *mockMetaMetricBatcher
		mockChainer  *mockBatchCounterChainer
		metricClient *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		mockDopplerServerA = newMockDopplerServer()
		mockDopplerServerB = newMockDopplerServer()
		mockFinder = newMockFinder()

		pool := plumbing.NewPool(2, grpc.WithInsecure())

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
		metricClient = testhelper.NewMetricClient()
		connector = plumbing.NewGRPCConnector(5, pool, mockFinder, mockBatcher, metricClient)

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

				dopplerA *MockDopplerServer
				dopplerB *MockDopplerServer
			)

			Context("when dopplers respond", func() {
				BeforeEach(func() {
					dopplerA = NewMockDopplerServer(testMetricA, testRecentLogA)
					dopplerB = NewMockDopplerServer(testMetricB, testRecentLogB)

					event := dopplerservice.Event{
						GRPCDopplers: []string{
							dopplerA.addr.String(),
							dopplerB.addr.String(),
						},
					}
					mockFinder.NextOutput.Ret0 <- event
				})

				AfterEach(func() {
					dopplerA.Stop()
					dopplerB.Stop()
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

			Context("when dopplers don't respond", func() {
				BeforeEach(func() {
					dopplerA = NewMockDopplerServer(testMetricA, testRecentLogA)
					dopplerB = NewMockDopplerServer(nil, nil)

					event := dopplerservice.Event{
						GRPCDopplers: []string{
							dopplerA.addr.String(),
							dopplerB.addr.String(),
						},
					}
					mockFinder.NextOutput.Ret0 <- event
				})

				AfterEach(func() {
					dopplerA.Stop()
					dopplerB.Stop()
				})

				It("can request container metrics", func() {
					f := func() [][]byte {
						c, _ := context.WithTimeout(ctx, 250*time.Millisecond)
						return connector.ContainerMetrics(c, "test-app-id")
					}
					Eventually(f).Should(ConsistOf([][]byte{testMetricA}))
				})

				It("can request recent logs", func() {
					f := func() [][]byte {
						c, _ := context.WithTimeout(ctx, 250*time.Millisecond)
						return connector.RecentLogs(c, "test-app-id")
					}
					Eventually(f).Should(ConsistOf([][]byte{testRecentLogA}))
				})

				It("emits a metric when container metrics times out", func() {
					f := func() [][]byte {
						c, _ := context.WithTimeout(ctx, 250*time.Millisecond)
						return connector.ContainerMetrics(c, "test-app-id")
					}
					Eventually(f).Should(ConsistOf([][]byte{testMetricA}))

					var envelope *v2.Envelope
					for _, e := range metricClient.GetEnvelopes("query_timeout") {
						if e.DeprecatedTags["query"].GetText() == "container_metrics" {
							envelope = e
						}
					}
					Expect(envelope).ToNot(BeNil())
					Expect(envelope.GetCounter().GetDelta()).To(Equal(uint64(1)))
				})

				It("emits a metric when container metrics times out", func() {
					f := func() [][]byte {
						c, _ := context.WithTimeout(ctx, 250*time.Millisecond)
						return connector.RecentLogs(c, "test-app-id")
					}
					Eventually(f).Should(ConsistOf([][]byte{testRecentLogA}))

					var envelope *v2.Envelope
					for _, e := range metricClient.GetEnvelopes("query_timeout") {
						if e.DeprecatedTags["query"].GetText() == "recent_logs" {
							envelope = e
						}
					}
					Expect(envelope).ToNot(BeNil())
					Expect(envelope.GetCounter().GetDelta()).To(Equal(uint64(1)))
				})
			})
		})
	})
})

type MockDopplerServer struct {
	addr       net.Addr
	grpcServer *grpc.Server

	containerMetric []byte
	recentLog       []byte
}

func NewMockDopplerServer(containerMetric, recentLog []byte) *MockDopplerServer {
	lis, err := net.Listen("tcp", "localhost:0")
	Expect(err).ToNot(HaveOccurred())

	mockServer := &MockDopplerServer{
		addr:            lis.Addr(),
		containerMetric: containerMetric,
		recentLog:       recentLog,
		grpcServer:      grpc.NewServer(),
	}

	plumbing.RegisterDopplerServer(mockServer.grpcServer, mockServer)

	go mockServer.grpcServer.Serve(lis)

	return mockServer
}

func (m *MockDopplerServer) Subscribe(*plumbing.SubscriptionRequest, plumbing.Doppler_SubscribeServer) error {
	return nil
}

func (m *MockDopplerServer) ContainerMetrics(context.Context, *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	if m.containerMetric == nil {
		time.Sleep(5 * time.Second)
	}

	cm := &plumbing.ContainerMetricsResponse{
		Payload: [][]byte{m.containerMetric},
	}

	return cm, nil
}

func (m *MockDopplerServer) RecentLogs(context.Context, *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	if m.recentLog == nil {
		time.Sleep(5 * time.Second)
	}

	rl := &plumbing.RecentLogsResponse{
		Payload: [][]byte{m.recentLog},
	}

	return rl, nil
}

func (m *MockDopplerServer) Stop() {
	m.grpcServer.Stop()
}

func readFromSubscription(ctx context.Context, req *plumbing.SubscriptionRequest, connector *plumbing.GRPCConnector) (<-chan []byte, <-chan error, chan struct{}) {
	data := make(chan []byte, 100)
	errs := make(chan error, 100)
	ready := make(chan struct{})

	go func() {
		defer GinkgoRecover()
		r, err := connector.Subscribe(ctx, req)
		close(ready)
		Expect(err).ToNot(HaveOccurred())
		for {
			d, e := r()
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
