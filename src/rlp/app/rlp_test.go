package app_test

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	app "code.cloudfoundry.org/loggregator/rlp/app"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Start", func() {
	It("receive messages via egress client", func(done Done) {
		defer close(done)
		_, _, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)
		defer func() {
			Expect(dopplerLis.Close()).To(Succeed())
		}()

		egressAddr, _ := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)

		egressStream, cleanup := setupRLPStream(egressAddr, testservers.LoggregatorTestCerts)
		defer cleanup()

		envelope, err := egressStream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.GetSourceId()).To(Equal("a"))
	}, 10)

	It("receive messages via egress batching client", func(done Done) {
		defer close(done)
		_, _, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)
		defer func() {
			Expect(dopplerLis.Close()).To(Succeed())
		}()

		egressAddr, _ := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)

		egressStream, cleanup := setupRLPBatchedStream(egressAddr, testservers.LoggregatorTestCerts)
		defer cleanup()

		f := func() int {
			batch, err := egressStream.Recv()
			Expect(err).ToNot(HaveOccurred())
			Expect(batch).ToNot(BeNil())
			return len(batch.GetBatch())
		}
		Eventually(f, 2).Should(BeNumerically(">", 10))
	}, 10)

	It("limits the number of allowed connections", func() {
		_, _, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)

		egressAddr, _ := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)
		createStream := func() error {
			egressClient, _ := setupRLPClient(egressAddr, testservers.LoggregatorTestCerts)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			c, err := egressClient.Receiver(ctx, &loggregator_v2.EgressRequest{})
			if err != nil {
				return err
			}

			_, err = c.Recv()

			select {
			case <-ctx.Done():
				// We don't care about the timeout errors
				return nil
			default:
				return err
			}
		}
		Eventually(createStream).Should(HaveOccurred())
		Expect(dopplerLis.Close()).To(Succeed())
	})

	It("upgrades a LegacySelector into a Selector", func() {
		_, doppler, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)
		defer dopplerLis.Close()

		egressAddr, _ := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)

		egressClient, cleanup := setupRLPClient(egressAddr, testservers.LoggregatorTestCerts)
		defer cleanup()

		Eventually(func() error {
			var err error
			_, err = egressClient.Receiver(context.Background(), &loggregator_v2.EgressRequest{
				LegacySelector: &loggregator_v2.Selector{
					SourceId: "some-id",
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			})
			return err
		}, 5).ShouldNot(HaveOccurred())

		Eventually(func() int {
			return len(doppler.requests())
		}).ShouldNot(BeZero())

		req := doppler.requests()[0]

		Expect(len(req.Selectors)).ToNot(BeZero())
		Expect(req.Selectors[0].SourceId).To(Equal("some-id"))
	})

	Describe("draining", func() {
		It("Stops accepting new connections", func() {
			_, _, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)
			defer func() {
				Expect(dopplerLis.Close()).To(Succeed())
			}()

			egressAddr, rlp := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)

			egressClient, cleanup := setupRLPClient(egressAddr, testservers.LoggregatorTestCerts)
			defer cleanup()

			Eventually(func() error {
				var err error
				_, err = egressClient.Receiver(context.Background(), &loggregator_v2.EgressRequest{})
				return err
			}, 5).ShouldNot(HaveOccurred())

			rlp.Stop()

			_, err := egressClient.Receiver(context.Background(), &loggregator_v2.EgressRequest{})
			Expect(err).To(HaveOccurred())
		})

		It("Drains the envelopes", func() {
			_, doppler, dopplerLis := setupDoppler(testservers.LoggregatorTestCerts)
			defer func() {
				Expect(dopplerLis.Close()).To(Succeed())
			}()
			egressAddr, rlp := setupRLP(dopplerLis, testservers.LoggregatorTestCerts)

			stream, cleanup := setupRLPStream(egressAddr, testservers.LoggregatorTestCerts)
			defer cleanup()

			Eventually(doppler.requests, 5).ShouldNot(BeEmpty())

			done := make(chan struct{})
			go func() {
				// Wait for envelope to reach buffer
				time.Sleep(250 * time.Millisecond)
				defer close(done)
				rlp.Stop()
			}()

			// Currently, the call to Stop() blocks.
			// We need to Recv the message after the call to Stop has been
			// made.
			_, err := stream.Recv()
			Expect(err).ToNot(HaveOccurred())

			errs := make(chan error, 1000)
			go func() {
				for {
					_, err := stream.Recv()
					errs <- err
				}
			}()
			Eventually(errs, 5).Should(Receive(HaveOccurred()))
			Eventually(done).Should(BeClosed())
		})
	})
})

func setupDoppler(testCerts *testservers.TestCerts) (*mockDopplerServer, *spyDopplerV2, net.Listener) {
	dopplerV1 := newMockDopplerServer()
	dopplerV2 := newSpyDopplerV2()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())

	tlsCredentials, err := plumbing.NewServerCredentials(
		testCerts.Cert("doppler"),
		testCerts.Key("doppler"),
		testCerts.CA(),
	)
	Expect(err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))
	plumbing.RegisterDopplerServer(grpcServer, dopplerV1)
	loggregator_v2.RegisterEgressServer(grpcServer, dopplerV2)
	go func() {
		log.Println(grpcServer.Serve(lis))
	}()
	return dopplerV1, dopplerV2, lis
}

func setupRLP(dopplerLis net.Listener, testCerts *testservers.TestCerts) (addr string, rlp *app.RLP) {
	ingressTLSCredentials, err := plumbing.NewClientCredentials(
		testCerts.Cert("reverselogproxy"),
		testCerts.Key("reverselogproxy"),
		testCerts.CA(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	egressTLSCredentials, err := plumbing.NewServerCredentials(
		testCerts.Cert("reverselogproxy"),
		testCerts.Key("reverselogproxy"),
		testCerts.CA(),
	)
	Expect(err).ToNot(HaveOccurred())

	rlp = app.NewRLP(
		testhelper.NewMetricClient(),
		app.WithEgressPort(0),
		app.WithIngressAddrs([]string{dopplerLis.Addr().String()}),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(ingressTLSCredentials)),
		app.WithEgressServerOptions(grpc.Creds(egressTLSCredentials)),
		app.WithMaxEgressConnections(1),
	)

	go rlp.Start()
	return rlp.EgressAddr().String(), rlp
}

func setupRLPClient(egressAddr string, testCerts *testservers.TestCerts) (loggregator_v2.EgressClient, func()) {
	ingressTLSCredentials, err := plumbing.NewClientCredentials(
		testCerts.Cert("reverselogproxy"),
		testCerts.Key("reverselogproxy"),
		testCerts.CA(),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(
		egressAddr,
		grpc.WithTransportCredentials(ingressTLSCredentials),
	)
	Expect(err).ToNot(HaveOccurred())

	egressClient := loggregator_v2.NewEgressClient(conn)

	return egressClient, func() {
		Expect(conn.Close()).To(Succeed())
	}
}

func setupRLPStream(egressAddr string, testCerts *testservers.TestCerts) (loggregator_v2.Egress_ReceiverClient, func()) {
	egressClient, cleanup := setupRLPClient(egressAddr, testCerts)

	var egressStream loggregator_v2.Egress_ReceiverClient
	Eventually(func() error {
		var err error
		egressStream, err = egressClient.Receiver(context.Background(), &loggregator_v2.EgressRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		})
		return err
	}, 5).ShouldNot(HaveOccurred())

	return egressStream, cleanup
}

func setupRLPBatchedStream(egressAddr string, testCerts *testservers.TestCerts) (loggregator_v2.Egress_BatchedReceiverClient, func()) {
	egressClient, cleanup := setupRLPClient(egressAddr, testCerts)

	var egressStream loggregator_v2.Egress_BatchedReceiverClient
	Eventually(func() error {
		var err error
		egressStream, err = egressClient.BatchedReceiver(context.Background(), &loggregator_v2.EgressBatchRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		})
		return err
	}, 5).ShouldNot(HaveOccurred())

	return egressStream, cleanup
}

type spyDopplerV2 struct {
	loggregator_v2.EgressServer

	mu        sync.Mutex
	_requests []*loggregator_v2.EgressBatchRequest

	batch []*loggregator_v2.Envelope
}

func newSpyDopplerV2() *spyDopplerV2 {
	return &spyDopplerV2{
		batch: []*loggregator_v2.Envelope{
			{SourceId: "a"},
			{SourceId: "b"},
			{SourceId: "c"},
		},
	}
}

func (s *spyDopplerV2) Receiver(*loggregator_v2.EgressRequest, loggregator_v2.Egress_ReceiverServer) error {
	return status.Errorf(codes.Unimplemented, "use BatchedReceiver")
}

func (s *spyDopplerV2) BatchedReceiver(r *loggregator_v2.EgressBatchRequest, b loggregator_v2.Egress_BatchedReceiverServer) error {
	s.mu.Lock()
	s._requests = append(s._requests, r)
	s.mu.Unlock()

	for {
		select {
		case <-b.Context().Done():
			return nil
		default:
			err := b.Send(&loggregator_v2.EnvelopeBatch{Batch: s.batch})
			if err != nil {
				return err
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *spyDopplerV2) requests() []*loggregator_v2.EgressBatchRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s._requests
}
