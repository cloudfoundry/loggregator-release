package plumbing_test

import (
	"log"
	"net"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("GRPCConnector", func() {
	var (
		req       *plumbing.SubscriptionRequest
		connector *plumbing.GRPCConnector

		mockDopplerServerA *spyRouter
		mockDopplerServerB *spyRouter
		mockFinder         *mockFinder

		metricClient *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		mockDopplerServerA = startMockDopplerServer()
		mockDopplerServerB = startMockDopplerServer()
		mockFinder = newMockFinder()

		pool := plumbing.NewPool(grpc.WithInsecure())

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
		mockDopplerServerA.Stop()
		mockDopplerServerB.Stop()
	})

	Describe("Subscribe()", func() {
		var (
			ctx        context.Context
			cancelFunc func()
		)

		BeforeEach(func() {
			ctx, cancelFunc = context.WithCancel(context.Background())
		})

		AfterEach(func() {
			cancelFunc()
		})

		Context("when no dopplers are available", func() {
			Context("when a doppler comes online after stream is established", func() {
				var (
					data <-chan []byte
					errs <-chan error
				)

				BeforeEach(func() {
					event := plumbing.Event{
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					var ready chan struct{}
					data, errs, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())

					mockFinder.NextOutput.Ret0 <- event
				})

				It("connects to the doppler with the correct request", func() {
					var r *plumbing.SubscriptionRequest
					Eventually(mockDopplerServerA.requests).Should(Receive(&r))
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
					Eventually(mockDopplerServerA.streams).Should(Receive())
					cancelFunc()

					newData, _, ready := readFromSubscription(context.Background(), req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.requests).Should(HaveLen(2))

					senderA := captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("some-data-a")},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(newData, 5).Should(Receive())
				})

				Context("when another doppler comes online", func() {
					var secondEvent plumbing.Event
					var mockDopplerServerC *spyRouter

					BeforeEach(func() {
						Eventually(mockDopplerServerA.requests).Should(HaveLen(1))
						Eventually(mockDopplerServerB.requests).Should(HaveLen(1))

						mockDopplerServerC = startMockDopplerServer()

						secondEvent = plumbing.Event{
							GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB, mockDopplerServerC),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent
					})

					AfterEach(func() {
						mockDopplerServerC.Stop()
					})

					It("does not reconnect to pre-existing dopplers", func() {
						Consistently(mockDopplerServerA.requests).Should(HaveLen(1))
						Consistently(mockDopplerServerB.requests).Should(HaveLen(1))
					})
				})

				Context("when a doppler is removed permanently", func() {
					var secondEvent plumbing.Event
					var listener net.Listener

					BeforeEach(func() {
						senderA := captureSubscribeSender(mockDopplerServerA)
						err := senderA.Send(&plumbing.BatchResponse{
							Payload: [][]byte{[]byte("some-data-a")},
						})
						Expect(err).ToNot(HaveOccurred())
						Eventually(data).Should(Receive(Equal([]byte("some-data-a"))))

						secondEvent = plumbing.Event{
							GRPCDopplers: createGrpcURIs(mockDopplerServerB),
						}
						mockFinder.NextOutput.Ret0 <- secondEvent

						mockDopplerServerA.Stop()
						time.Sleep(1 * time.Second)
						listener = startListener(mockDopplerServerA.addr.String())
					})

					AfterEach(func() {
						listener.Close()
					})

					It("does not attempt reconnect", func() {
						c := make(chan net.Conn, 100)
						go func(lis net.Listener) {
							conn, _ := lis.Accept()
							c <- conn
						}(listener)
						Consistently(c, 2).ShouldNot(Receive())
					})
				})

				Context("when the consumer disconnects", func() {
					BeforeEach(func() {
						cancelFunc()
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
							GRPCDopplers: []string{},
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
								GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
							}

							mockFinder.NextOutput.Ret0 <- thirdEvent
						})

						It("doesn't create a second connection", func() {
							Consistently(mockDopplerServerA.requests, 1).Should(HaveLen(1))
						})

						Context("when the first connection is closed", func() {
							var (
								newCtx        context.Context
								newCancelFunc func()
							)

							BeforeEach(func() {
								cancelFunc()

								newCtx, newCancelFunc = context.WithCancel(context.Background())
							})

							AfterEach(func() {
								newCancelFunc()
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
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					mockFinder.NextOutput.Ret0 <- event
					Eventually(mockFinder.NextCalled).Should(HaveLen(2))

					_, _, ready := readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
				})

				It("connects to the doppler with the correct request", func() {
					var r *plumbing.SubscriptionRequest
					Eventually(mockDopplerServerA.requests).Should(Receive(&r))
					Expect(proto.Equal(r, req)).To(BeTrue())
				})
			})

			Context("when new doppler is not available right away", func() {
				var (
					event    plumbing.Event
					listener net.Listener
				)

				BeforeEach(func() {
					event = plumbing.Event{
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					mockDopplerServerA.Stop()
					mockFinder.NextOutput.Ret0 <- event

					_, _, _ = readFromSubscription(ctx, req, connector)
					Eventually(mockDopplerServerB.requests).ShouldNot(HaveLen(0))

					listener = startListener(mockDopplerServerA.addr.String())

					mockDopplerServerA.grpcServer = grpc.NewServer()
					plumbing.RegisterDopplerServer(mockDopplerServerA.grpcServer, mockDopplerServerA)
					go func() {
						log.Println(mockDopplerServerA.grpcServer.Serve(listener))
					}()
				})

				It("attempts to reconnect", func() {
					Eventually(mockDopplerServerA.streams, 5).Should(Receive())
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
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					mockFinder.NextOutput.Ret0 <- event

					data, _, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.requests).Should(HaveLen(1))
					Eventually(mockDopplerServerB.requests).Should(HaveLen(1))

					senderA = captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&plumbing.BatchResponse{
						Payload: [][]byte{[]byte("test-payload-1")},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(data).Should(Receive(Equal([]byte("test-payload-1"))))

					mockDopplerServerA.Stop()
					mockDopplerServerA = startMockDopplerServer()

					event = plumbing.Event{
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					mockFinder.NextOutput.Ret0 <- event
				})

				It("continues sending", func() {
					senderA = captureSubscribeSender(mockDopplerServerA)
					Eventually(func() error {
						return senderA.Send(&plumbing.BatchResponse{
							Payload: [][]byte{[]byte("test-payload-2")},
						})
					}, 5).ShouldNot(HaveOccurred())

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

func createGrpcURIs(ms ...*spyRouter) []string {
	var results []string
	for _, m := range ms {
		results = append(results, m.addr.String())
	}
	return results
}

func captureSubscribeSender(doppler *spyRouter) plumbing.Doppler_BatchSubscribeServer {
	var server plumbing.Doppler_BatchSubscribeServer
	EventuallyWithOffset(1, doppler.streams, 5).Should(Receive(&server))
	return server
}

type spyRouter struct {
	plumbing.DopplerServer

	addr       net.Addr
	grpcServer *grpc.Server
	requests   chan *plumbing.SubscriptionRequest
	streams    chan plumbing.Doppler_BatchSubscribeServer
}

func startMockDopplerServer() *spyRouter {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())

	mockServer := &spyRouter{
		addr:       lis.Addr(),
		grpcServer: grpc.NewServer(),
		requests:   make(chan *plumbing.SubscriptionRequest, 100),
		streams:    make(chan plumbing.Doppler_BatchSubscribeServer, 100),
	}

	plumbing.RegisterDopplerServer(mockServer.grpcServer, mockServer)

	go func() {
		log.Println(mockServer.grpcServer.Serve(lis))
	}()

	return mockServer
}

func (m *spyRouter) Subscribe(req *plumbing.SubscriptionRequest, stream plumbing.Doppler_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "Use batched receiver")
}

func (m *spyRouter) BatchSubscribe(req *plumbing.SubscriptionRequest, stream plumbing.Doppler_BatchSubscribeServer) error {
	m.requests <- req
	m.streams <- stream

	<-stream.Context().Done()

	return nil
}

func (m *spyRouter) Stop() {
	m.grpcServer.Stop()
}
