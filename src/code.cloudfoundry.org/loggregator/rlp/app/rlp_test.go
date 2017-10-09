package app_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/testservers"

	app "code.cloudfoundry.org/loggregator/rlp/app"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Start", func() {
	It("receive messages via egress client", func(done Done) {
		defer close(done)
		_, _, dopplerLis := setupDoppler()
		defer func() {
			Expect(dopplerLis.Close()).To(Succeed())
		}()

		egressAddr, _ := setupRLP(dopplerLis, "localhost:0")

		egressStream, cleanup := setupRLPStream(egressAddr)
		defer cleanup()

		envelope, err := egressStream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.GetSourceId()).To(Equal("a"))
	}, 10)

	It("receive messages via egress batching client", func(done Done) {
		defer close(done)
		_, _, dopplerLis := setupDoppler()
		defer func() {
			Expect(dopplerLis.Close()).To(Succeed())
		}()

		egressAddr, _ := setupRLP(dopplerLis, "localhost:0")

		egressStream, cleanup := setupRLPBatchedStream(egressAddr)
		defer cleanup()

		f := func() int {
			batch, err := egressStream.Recv()
			Expect(err).ToNot(HaveOccurred())
			return len(batch.GetBatch())
		}
		Eventually(f, 2).Should(Equal(100))
	}, 10)

	It("receives container metrics via egress query client", func() {
		doppler, _, dopplerLis := setupDoppler()
		doppler.ContainerMetricsOutput.Err <- nil
		doppler.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
			Payload: [][]byte{buildContainerMetric()},
		}

		egressAddr, _ := setupRLP(dopplerLis, "localhost:0")
		egressClient, cleanup := setupRLPQueryClient(egressAddr)
		defer cleanup()

		var resp *v2.QueryResponse
		f := func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			var err error
			resp, err = egressClient.ContainerMetrics(ctx, &v2.ContainerMetricRequest{
				SourceId: "some-app",
			})

			return err
		}
		Eventually(f).Should(Succeed())

		Expect(resp.Envelopes).To(HaveLen(1))
		Expect(dopplerLis.Close()).To(Succeed())
	})

	It("limits the number of allowed connections", func() {
		doppler, _, dopplerLis := setupDoppler()
		doppler.ContainerMetricsOutput.Err <- nil
		doppler.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
			Payload: [][]byte{buildContainerMetric()},
		}

		egressAddr, _ := setupRLP(dopplerLis, "localhost:0")
		createStream := func() error {
			egressClient, _ := setupRLPClient(egressAddr)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			c, err := egressClient.Receiver(ctx, &v2.EgressRequest{})
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

	Describe("draining", func() {
		It("Stops accepting new connections", func() {
			doppler, _, dopplerLis := setupDoppler()
			doppler.ContainerMetricsOutput.Err <- nil
			doppler.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
				Payload: [][]byte{buildContainerMetric()},
			}

			egressAddr, rlp := setupRLP(dopplerLis, "localhost:0")

			egressClient, cleanup := setupRLPQueryClient(egressAddr)
			defer cleanup()

			Eventually(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				_, err := egressClient.ContainerMetrics(ctx, &v2.ContainerMetricRequest{
					SourceId: "some-id",
				})
				return err
			}, 5).ShouldNot(HaveOccurred())

			rlp.Stop()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := egressClient.ContainerMetrics(ctx, &v2.ContainerMetricRequest{
				SourceId: "some-id",
			})
			Expect(err).To(HaveOccurred())
			Expect(ctx.Done()).ToNot(BeClosed())
			Expect(dopplerLis.Close()).To(Succeed())
		})

		It("Drains the envelopes", func() {
			_, doppler, dopplerLis := setupDoppler()
			defer func() {
				Expect(dopplerLis.Close()).To(Succeed())
			}()
			egressAddr, rlp := setupRLP(dopplerLis, "localhost:0")

			stream, cleanup := setupRLPStream(egressAddr)
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

	Describe("health endpoint", func() {
		It("returns health metrics", func() {
			doppler, _, dopplerLis := setupDoppler()
			doppler.ContainerMetricsOutput.Err <- nil
			doppler.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
				Payload: [][]byte{buildContainerMetric()},
			}
			setupRLP(dopplerLis, "localhost:8080")

			var err error
			var resp *http.Response
			Eventually(func() error {
				resp, err = http.Get(fmt.Sprintf("http://localhost:8080/health"))
				return err
			}).Should(Succeed())
			defer func() {
				Expect(resp.Body.Close()).To(Succeed())
			}()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("subscriptionCount"))
			Expect(dopplerLis.Close()).To(Succeed())
		})
	})
})

func buildLogMessage() [][]byte {
	e := &events.Envelope{
		Origin:    proto.String("some-origin"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("foo"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String("test-app"),
		},
	}
	b, _ := proto.Marshal(e)
	return [][]byte{b}
}

func buildContainerMetric() []byte {
	e := &events.Envelope{
		Origin:    proto.String("some-origin"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("test-app"),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(10.0),
			MemoryBytes:   proto.Uint64(1),
			DiskBytes:     proto.Uint64(1),
		},
	}
	b, _ := proto.Marshal(e)
	return b
}

func setupDoppler() (*mockDopplerServer, *spyDopplerV2, net.Listener) {
	dopplerV1 := newMockDopplerServer()
	dopplerV2 := newSpyDopplerV2()

	lis, err := net.Listen("tcp", "localhost:0")
	Expect(err).ToNot(HaveOccurred())

	tlsCredentials, err := plumbing.NewServerCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))
	plumbing.RegisterDopplerServer(grpcServer, dopplerV1)
	v2.RegisterEgressServer(grpcServer, dopplerV2)
	go func() {
		log.Println(grpcServer.Serve(lis))
	}()
	return dopplerV1, dopplerV2, lis
}

func setupRLP(dopplerLis net.Listener, healthAddr string) (addr string, rlp *app.RLP) {
	ingressTLSCredentials, err := plumbing.NewClientCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	egressTLSCredentials, err := plumbing.NewServerCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())

	rlp = app.NewRLP(
		testhelper.NewMetricClient(),
		app.WithEgressPort(0),
		app.WithIngressAddrs([]string{dopplerLis.Addr().String()}),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(ingressTLSCredentials)),
		app.WithEgressServerOptions(grpc.Creds(egressTLSCredentials)),
		app.WithMaxEgressConnections(1),
		app.WithHealthAddr(healthAddr),
	)

	go rlp.Start()
	return rlp.EgressAddr().String(), rlp
}

func setupRLPClient(egressAddr string) (v2.EgressClient, func()) {
	ingressTLSCredentials, err := plumbing.NewClientCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(
		egressAddr,
		grpc.WithTransportCredentials(ingressTLSCredentials),
	)
	Expect(err).ToNot(HaveOccurred())

	egressClient := v2.NewEgressClient(conn)

	return egressClient, func() {
		Expect(conn.Close()).To(Succeed())
	}
}

func setupRLPStream(egressAddr string) (v2.Egress_ReceiverClient, func()) {
	egressClient, cleanup := setupRLPClient(egressAddr)

	var egressStream v2.Egress_ReceiverClient
	Eventually(func() error {
		var err error
		egressStream, err = egressClient.Receiver(context.Background(), &v2.EgressRequest{})
		return err
	}, 5).ShouldNot(HaveOccurred())

	return egressStream, cleanup
}

func setupRLPBatchedStream(egressAddr string) (v2.Egress_BatchedReceiverClient, func()) {
	egressClient, cleanup := setupRLPClient(egressAddr)

	var egressStream v2.Egress_BatchedReceiverClient
	Eventually(func() error {
		var err error
		egressStream, err = egressClient.BatchedReceiver(context.Background(), &v2.EgressBatchRequest{})
		return err
	}, 5).ShouldNot(HaveOccurred())

	return egressStream, cleanup
}

func setupRLPQueryClient(egressAddr string) (v2.EgressQueryClient, func()) {
	ingressTLSCredentials, err := plumbing.NewClientCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(
		egressAddr,
		grpc.WithTransportCredentials(ingressTLSCredentials),
	)
	Expect(err).ToNot(HaveOccurred())

	egressClient := v2.NewEgressQueryClient(conn)

	return egressClient, func() {
		Expect(conn.Close()).To(Succeed())
	}
}

type spyDopplerV2 struct {
	mu        sync.Mutex
	_requests []*v2.EgressBatchRequest

	batch []*v2.Envelope
}

func newSpyDopplerV2() *spyDopplerV2 {
	return &spyDopplerV2{
		batch: []*v2.Envelope{
			{SourceId: "a"},
			{SourceId: "b"},
			{SourceId: "c"},
		},
	}
}

func (s *spyDopplerV2) Receiver(*v2.EgressRequest, v2.Egress_ReceiverServer) error {
	return grpc.Errorf(codes.Unimplemented, "use BatchedReceiver")
}

func (s *spyDopplerV2) BatchedReceiver(r *v2.EgressBatchRequest, b v2.Egress_BatchedReceiverServer) error {
	s.mu.Lock()
	s._requests = append(s._requests, r)
	s.mu.Unlock()

	for {
		select {
		case <-b.Context().Done():
			return nil
		default:
			err := b.Send(&v2.EnvelopeBatch{Batch: s.batch})
			if err != nil {
				return err
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *spyDopplerV2) requests() []*v2.EgressBatchRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s._requests
}
