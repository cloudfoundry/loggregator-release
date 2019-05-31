package ingress_test

import (
	"log"
	"net"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/rlp/internal/ingress"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCConnector", func() {
	var (
		req       *loggregator_v2.EgressBatchRequest
		connector *ingress.GRPCConnector

		mockDopplerServerA *spyRouter
		mockDopplerServerB *spyRouter
		mockFinder         *mockFinder

		metricClient *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		mockDopplerServerA = startMockDopplerServer()
		mockDopplerServerB = startMockDopplerServer()
		mockFinder = newMockFinder()

		pool := ingress.NewPool(2, grpc.WithInsecure())

		req = &loggregator_v2.EgressBatchRequest{
			ShardId: "test-sub-id",
			LegacySelector: &loggregator_v2.Selector{
				SourceId: "test-app-id",
			},
		}
		metricClient = testhelper.NewMetricClient()
		connector = ingress.NewGRPCConnector(5, pool, mockFinder, metricClient)
	})

	AfterEach(func() {
		mockDopplerServerA.Stop()
		mockDopplerServerB.Stop()
	})

	Describe("Subscribe()", func() {
		var (
			ctx       context.Context
			cancelCtx func()
		)

		BeforeEach(func() {
			ctx, cancelCtx = context.WithCancel(context.Background())
		})

		Context("when a doppler comes online after stream is established", func() {
			var (
				data <-chan *loggregator_v2.Envelope
			)

			BeforeEach(func() {
				event := plumbing.Event{
					GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
				}

				var ready chan struct{}
				data, _, ready = readFromSubscription(ctx, req, connector)
				Eventually(ready).Should(BeClosed())

				mockFinder.NextOutput.Ret0 <- event
			})

			It("connects to the doppler with the correct request", func() {
				var r *loggregator_v2.EgressBatchRequest
				Eventually(mockDopplerServerA.requests).Should(Receive(&r))
				Expect(proto.Equal(r, req)).To(BeTrue())
			})

			It("returns data from both dopplers", func() {
				senderA := captureSubscribeSender(mockDopplerServerA)
				senderB := captureSubscribeSender(mockDopplerServerB)

				err := senderA.Send(&loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{{SourceId: "A"}, {SourceId: "B"}},
				})
				Expect(err).ToNot(HaveOccurred())
				Eventually(data).Should(Receive(Equal(&loggregator_v2.Envelope{SourceId: "A"})))
				Eventually(data).Should(Receive(Equal(&loggregator_v2.Envelope{SourceId: "B"})))

				err = senderB.Send(&loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{{SourceId: "C"}, {SourceId: "D"}},
				})
				Expect(err).ToNot(HaveOccurred())
				Eventually(data).Should(Receive(Equal(&loggregator_v2.Envelope{SourceId: "C"})))
				Eventually(data).Should(Receive(Equal(&loggregator_v2.Envelope{SourceId: "D"})))
			})

			It("does not close the doppler connection when a client exits", func() {
				Eventually(mockDopplerServerA.servers).Should(Receive())
				cancelCtx()

				newData, _, ready := readFromSubscription(context.Background(), req, connector)
				Eventually(ready).Should(BeClosed())
				Eventually(mockDopplerServerA.requests).Should(HaveLen(2))

				senderA := captureSubscribeSender(mockDopplerServerA)
				err := senderA.Send(&loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{{SourceId: "A"}},
				})
				Expect(err).ToNot(HaveOccurred())

				Eventually(newData, 5).Should(Receive())
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
				var r *loggregator_v2.EgressBatchRequest
				Eventually(mockDopplerServerA.requests).Should(Receive(&r))
				Expect(proto.Equal(r, req)).To(BeTrue())
			})

			It("increments a counter for total doppler connections", func() {
				Eventually(func() uint64 {
					return metricClient.GetDelta("log_router_connects")
				}).Should(Equal(uint64(2)))
			})
		})

		Context("when a doppler disconnects", func() {
			BeforeEach(func() {
				event := plumbing.Event{
					GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
				}
				mockFinder.NextOutput.Ret0 <- event

				_, _, ready := readFromSubscription(ctx, req, connector)
				Eventually(ready).Should(BeClosed())

				Eventually(mockDopplerServerA.requests).Should(Receive())
				Eventually(mockDopplerServerB.requests).Should(Receive())
			})

			It("emits a counter metric for total doppler disconnects", func() {
				cancelCtx()

				Eventually(func() uint64 {
					return metricClient.GetDelta("log_router_disconnects")
				}, 5).Should(Equal(uint64(2)))
			})
		})
	})
})

type spyRouter struct {
	addr       net.Addr
	grpcServer *grpc.Server
	requests   chan *loggregator_v2.EgressBatchRequest
	servers    chan loggregator_v2.Egress_BatchedReceiverServer
}

func startMockDopplerServer() *spyRouter {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())

	mockServer := &spyRouter{
		addr:       lis.Addr(),
		grpcServer: grpc.NewServer(),
		requests:   make(chan *loggregator_v2.EgressBatchRequest, 100),
		servers:    make(chan loggregator_v2.Egress_BatchedReceiverServer, 100),
	}

	loggregator_v2.RegisterEgressServer(mockServer.grpcServer, mockServer)

	go func() {
		log.Println(mockServer.grpcServer.Serve(lis))
	}()

	return mockServer
}

func (m *spyRouter) Receiver(*loggregator_v2.EgressRequest, loggregator_v2.Egress_ReceiverServer) error {
	return status.Errorf(codes.Unimplemented, "Use batched receiver")
}

func (m *spyRouter) BatchedReceiver(req *loggregator_v2.EgressBatchRequest, s loggregator_v2.Egress_BatchedReceiverServer) error {
	m.requests <- req
	m.servers <- s

	<-s.Context().Done()

	return nil
}

func (m *spyRouter) Stop() {
	m.grpcServer.Stop()
}

func readFromSubscription(ctx context.Context, req *loggregator_v2.EgressBatchRequest, connector *ingress.GRPCConnector) (<-chan *loggregator_v2.Envelope, <-chan error, chan struct{}) {
	data := make(chan *loggregator_v2.Envelope, 100)
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

func captureSubscribeSender(doppler *spyRouter) loggregator_v2.Egress_BatchedReceiverServer {
	var server loggregator_v2.Egress_BatchedReceiverServer
	EventuallyWithOffset(1, doppler.servers, 5).Should(Receive(&server))
	return server
}

type mockFinder struct {
	NextCalled chan bool
	NextOutput struct {
		Ret0 chan plumbing.Event
	}
}

func newMockFinder() *mockFinder {
	m := &mockFinder{}
	m.NextCalled = make(chan bool, 100)
	m.NextOutput.Ret0 = make(chan plumbing.Event, 100)
	return m
}
func (m *mockFinder) Next() plumbing.Event {
	m.NextCalled <- true
	return <-m.NextOutput.Ret0
}
