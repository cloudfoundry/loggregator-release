package plumbing_test

import (
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/gogo/protobuf/proto"
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

		metricClient *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		mockDopplerServerA = newMockDopplerServer()
		mockDopplerServerB = newMockDopplerServer()
		mockFinder = newMockFinder()

		pool := plumbing.NewPool(2, grpc.WithInsecure())

		lisA, serverA := startGRPCServer(mockDopplerServerA, "127.0.0.1:0")
		lisB, serverB := startGRPCServer(mockDopplerServerB, "127.0.0.1:0")
		listeners = append(listeners, lisA, lisB)
		grpcServers = append(grpcServers, serverA, serverB)

		req = &plumbing.SubscriptionRequest{
			ShardID: "test-sub-id",
			Filter: &plumbing.Filter{
				AppID: "test-app-id",
			},
		}
		metricClient = testhelper.NewMetricClient()
		connector = plumbing.NewGRPCConnector(5, pool, mockFinder, metricClient)
	})

	AfterEach(func() {
		for _, lis := range listeners {
			Expect(lis.Close()).To(Succeed())
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
					event := plumbing.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					var ready chan struct{}
					data, errs, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())

					mockFinder.NextOutput.Ret0 <- event
				})

				It("connects to the doppler with the correct request", func() {
					var r *plumbing.SubscriptionRequest
					Eventually(mockDopplerServerA.BatchSubscribeInput.Req).Should(Receive(&r))
					Expect(proto.Equal(r, req)).To(BeTrue())
				})

				It("returns data from both dopplers", func() {
					senderA := captureSubscribeSender(mockDopplerServerA)
					senderB := captureSubscribeSender(mockDopplerServerB)

					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("some-data-a")},
					})
					Expect(err).ToNot(HaveOccurred())
					Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))

					err = senderB.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("some-data-b")},
					})
					Expect(err).ToNot(HaveOccurred())
					Eventually(data).Should(Receive(Equal([]byte("some-data-b"))))
				})

				It("does not close the doppler connection when a client exits", func() {
					Eventually(mockDopplerServerA.BatchSubscribeInput.Stream).Should(Receive())
					cancelCtx()

					newData, _, ready := readFromSubscription(context.Background(), req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.BatchSubscribeCalled).Should(HaveLen(2))

					senderA := captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("some-data-a")},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(newData, 5).Should(Receive())
				})

				Context("when another doppler comes online", func() {
					var secondEvent plumbing.Event

					BeforeEach(func() {
						Eventually(mockDopplerServerA.BatchSubscribeCalled).Should(HaveLen(1))
						Eventually(mockDopplerServerB.BatchSubscribeCalled).Should(HaveLen(1))

						mockDopplerServerC := newMockDopplerServer()
						lisC, serverC := startGRPCServer(mockDopplerServerC, "127.0.0.1:0")
						listeners = append(listeners, lisC)
						grpcServers = append(grpcServers, serverC)

						secondEvent = plumbing.Event{
							GRPCDopplers: createGrpcURIs(listeners),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent
					})

					It("does not reconnect to pre-existing dopplers", func() {
						Consistently(mockDopplerServerA.BatchSubscribeCalled).Should(HaveLen(1))
						Consistently(mockDopplerServerB.BatchSubscribeCalled).Should(HaveLen(1))
					})
				})

				Context("when a doppler is removed permanently", func() {
					var secondEvent plumbing.Event

					BeforeEach(func() {
						senderA := captureSubscribeSender(mockDopplerServerA)
						err := senderA.Send(&plumbing.BatchResponse{
							Payload: [][]byte{[]byte("some-data-a")},
						})
						Expect(err).ToNot(HaveOccurred())
						Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))

						secondEvent = plumbing.Event{
							GRPCDopplers: createGrpcURIs(listeners[1:]),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent

						Expect(listeners[0].Close()).To(Succeed())
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
						secondEvent plumbing.Event
						senderA     plumbing.Doppler_BatchSubscribeServer
					)
					BeforeEach(func() {
						senderA = captureSubscribeSender(mockDopplerServerA)

						secondEvent = plumbing.Event{
							GRPCDopplers: createGrpcURIs(nil),
						}

						mockFinder.NextOutput.Ret0 <- secondEvent
					})

					It("continues reading from the doppler", func() {
						err := senderA.Send(&plumbing.BatchResponse{
							Payload: [][]byte{[]byte("some-data-a")},
						})
						Expect(err).ToNot(HaveOccurred())
						Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))
					})

					It("accepts new streams", func() {
						data, _, ready := readFromSubscription(ctx, req, connector)
						Eventually(ready).Should(BeClosed())
						sender := captureSubscribeSender(mockDopplerServerA)
						err := sender.Send(&plumbing.BatchResponse{
							Payload: [][]byte{[]byte("some-data")},
						})
						Expect(err).ToNot(HaveOccurred())
						Eventually(data).Should(Receive())
					})

					Context("finder event readvertises the doppler", func() {
						var (
							thirdEvent plumbing.Event
						)

						BeforeEach(func() {
							thirdEvent = plumbing.Event{
								GRPCDopplers: createGrpcURIs(listeners),
							}

							mockFinder.NextOutput.Ret0 <- thirdEvent
						})

						It("doesn't create a second connection", func() {
							Consistently(mockDopplerServerA.BatchSubscribeCalled, 1).Should(HaveLen(1))
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
								err := sender.Send(&plumbing.BatchResponse{
									Payload: [][]byte{[]byte("some-data")},
								})
								Expect(err).ToNot(HaveOccurred())
								Eventually(data).Should(Receive())
							})
						})
					})
				})
			})

			Context("when a doppler comes online before stream has established", func() {
				var event plumbing.Event

				BeforeEach(func() {
					event = plumbing.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					mockFinder.NextOutput.Ret0 <- event
					Eventually(mockFinder.NextCalled).Should(HaveLen(2))

					_, _, ready := readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
				})

				It("connects to the doppler with the correct request", func() {
					var r *plumbing.SubscriptionRequest
					Eventually(mockDopplerServerA.BatchSubscribeInput.Req).Should(Receive(&r))
					Expect(proto.Equal(r, req)).To(BeTrue())
				})
			})

			Context("when new doppler is not available right away", func() {
				var (
					event plumbing.Event
				)

				BeforeEach(func() {
					event = plumbing.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					Expect(listeners[0].Close()).To(Succeed())
					grpcServers[0].Stop()

					mockFinder.NextOutput.Ret0 <- event

					_, _, _ = readFromSubscription(ctx, req, connector)
					Eventually(mockDopplerServerB.BatchSubscribeCalled).ShouldNot(HaveLen(0))
					listeners[0], grpcServers[0] = startGRPCServer(mockDopplerServerA, listeners[0].Addr().String())
				})

				It("attempts to reconnect", func() {
					Eventually(mockDopplerServerA.BatchSubscribeInput.Stream, 5).Should(Receive())
				})
			})

			Context("when an active doppler closes after reading", func() {
				var (
					event   plumbing.Event
					data    <-chan []byte
					ready   <-chan struct{}
					senderA plumbing.Doppler_BatchSubscribeServer
				)

				BeforeEach(func() {
					event = plumbing.Event{
						GRPCDopplers: createGrpcURIs(listeners),
					}

					mockFinder.NextOutput.Ret0 <- event

					data, _, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.BatchSubscribeCalled).Should(HaveLen(1))
					Eventually(mockDopplerServerB.BatchSubscribeCalled).Should(HaveLen(1))

					senderA = captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("test-payload-1")},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(data).Should(Receive(Equal([]byte("test-payload-1"))))

					Expect(listeners[0].Close()).To(Succeed())
					grpcServers[0].Stop()
					listeners[0], grpcServers[0] = startGRPCServer(mockDopplerServerA, listeners[0].Addr().String())
				})

				It("attempts to reconnect", func() {
					senderA = captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("test-payload-2")},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(data, 5).Should(Receive(Equal([]byte("test-payload-2"))))
				})
			})
		})
	})
})

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

func captureSubscribeSender(doppler *mockDopplerServer) plumbing.Doppler_BatchSubscribeServer {
	var server plumbing.Doppler_BatchSubscribeServer
	EventuallyWithOffset(1, doppler.BatchSubscribeInput.Stream, 5).Should(Receive(&server))
	return server
}
