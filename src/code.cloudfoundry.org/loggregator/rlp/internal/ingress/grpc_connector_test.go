package ingress_test

import (
	"log"
	"net"

	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/rlp/internal/ingress"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCConnector", func() {
	var (
		req       *v2.EgressBatchRequest
		connector *ingress.GRPCConnector

		mockDopplerServerA *MockDopplerServer
		mockDopplerServerB *MockDopplerServer
		mockFinder         *mockFinder

		metricClient *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		mockDopplerServerA = startMockDopplerServer()
		mockDopplerServerB = startMockDopplerServer()
		mockFinder = newMockFinder()

		pool := ingress.NewPool(2, grpc.WithInsecure())

		req = &v2.EgressBatchRequest{
			ShardId: "test-sub-id",
			LegacySelector: &v2.Selector{
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

		Context("when no dopplers are available", func() {
			Context("when a doppler comes online after stream is established", func() {
				var (
					data <-chan *v2.Envelope
					errs <-chan error
				)

				BeforeEach(func() {
					event := dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					var ready chan struct{}
					data, errs, ready = readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())

					mockFinder.NextOutput.Ret0 <- event
				})

				It("connects to the doppler with the correct request", func() {
					Eventually(mockDopplerServerA.requests).Should(
						Receive(Equal(req)),
					)
				})

				It("returns data from both dopplers", func() {
					senderA := captureSubscribeSender(mockDopplerServerA)
					senderB := captureSubscribeSender(mockDopplerServerB)

					err := senderA.Send(&v2.EnvelopeBatch{
						Batch: []*v2.Envelope{{SourceId: "A"}, {SourceId: "B"}},
					})
					Expect(err).ToNot(HaveOccurred())
					Eventually(data).Should(Receive(Equal(&v2.Envelope{SourceId: "A"})))
					Eventually(data).Should(Receive(Equal(&v2.Envelope{SourceId: "B"})))

					err = senderB.Send(&v2.EnvelopeBatch{
						Batch: []*v2.Envelope{{SourceId: "C"}, {SourceId: "D"}},
					})
					Expect(err).ToNot(HaveOccurred())
					Eventually(data).Should(Receive(Equal(&v2.Envelope{SourceId: "C"})))
					Eventually(data).Should(Receive(Equal(&v2.Envelope{SourceId: "D"})))
				})

				It("does not close the doppler connection when a client exits", func() {
					Eventually(mockDopplerServerA.servers).Should(Receive())
					cancelCtx()

					newData, _, ready := readFromSubscription(context.Background(), req, connector)
					Eventually(ready).Should(BeClosed())
					Eventually(mockDopplerServerA.requests).Should(HaveLen(2))

					senderA := captureSubscribeSender(mockDopplerServerA)
					err := senderA.Send(&v2.EnvelopeBatch{
						Batch: []*v2.Envelope{{SourceId: "A"}},
					})
					Expect(err).ToNot(HaveOccurred())

					Eventually(newData, 5).Should(Receive())
				})
			})

			Context("when a doppler comes online before stream has established", func() {
				var event dopplerservice.Event

				BeforeEach(func() {
					event = dopplerservice.Event{
						GRPCDopplers: createGrpcURIs(mockDopplerServerA, mockDopplerServerB),
					}

					mockFinder.NextOutput.Ret0 <- event
					Eventually(mockFinder.NextCalled).Should(HaveLen(2))

					_, _, ready := readFromSubscription(ctx, req, connector)
					Eventually(ready).Should(BeClosed())
				})

				It("connects to the doppler with the correct request", func() {
					Eventually(mockDopplerServerA.requests).Should(
						BeCalled(With(req, Not(BeNil()))),
					)
				})
			})
		})
	})
})

// TODO: Rename to spy and remove doppler word
type MockDopplerServer struct {
	addr       net.Addr
	grpcServer *grpc.Server
	requests   chan *v2.EgressBatchRequest
	servers    chan v2.Egress_BatchedReceiverServer
}

func startMockDopplerServer() *MockDopplerServer {
	lis, err := net.Listen("tcp", "localhost:0")
	Expect(err).ToNot(HaveOccurred())

	mockServer := &MockDopplerServer{
		addr:       lis.Addr(),
		grpcServer: grpc.NewServer(),
		requests:   make(chan *v2.EgressBatchRequest, 100),
		servers:    make(chan v2.Egress_BatchedReceiverServer, 100),
	}

	v2.RegisterEgressServer(mockServer.grpcServer, mockServer)

	go func() {
		log.Println(mockServer.grpcServer.Serve(lis))
	}()

	return mockServer
}

func (m *MockDopplerServer) Receiver(*v2.EgressRequest, v2.Egress_ReceiverServer) error {
	return grpc.Errorf(codes.Unimplemented, "Use batched receiver")
}

func (m *MockDopplerServer) BatchedReceiver(req *v2.EgressBatchRequest, s v2.Egress_BatchedReceiverServer) error {
	m.requests <- req
	m.servers <- s

	<-s.Context().Done()

	return nil
}

func (m *MockDopplerServer) Stop() {
	m.grpcServer.Stop()
}

func readFromSubscription(ctx context.Context, req *v2.EgressBatchRequest, connector *ingress.GRPCConnector) (<-chan *v2.Envelope, <-chan error, chan struct{}) {
	data := make(chan *v2.Envelope, 100)
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

func createGrpcURIs(ms ...*MockDopplerServer) []string {
	var results []string
	for _, m := range ms {
		results = append(results, m.addr.String())
	}
	return results
}

func captureSubscribeSender(doppler *MockDopplerServer) v2.Egress_BatchedReceiverServer {
	var server v2.Egress_BatchedReceiverServer
	EventuallyWithOffset(1, doppler.servers, 5).Should(Receive(&server))
	return server
}

type mockFinder struct {
	NextCalled chan bool
	NextOutput struct {
		Ret0 chan dopplerservice.Event
	}
}

func newMockFinder() *mockFinder {
	m := &mockFinder{}
	m.NextCalled = make(chan bool, 100)
	m.NextOutput.Ret0 = make(chan dopplerservice.Event, 100)
	return m
}
func (m *mockFinder) Next() dopplerservice.Event {
	m.NextCalled <- true
	return <-m.NextOutput.Ret0
}
