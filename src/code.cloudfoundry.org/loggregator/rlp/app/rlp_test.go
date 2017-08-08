package app_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/testservers"

	app "code.cloudfoundry.org/loggregator/rlp/app"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Start", func() {
	It("receive messages via egress client", func() {
		doppler, dopplerLis := setupDoppler()

		egressAddr, _ := setupRLP(dopplerLis, "localhost:0")

		egressStream, cleanup := setupRLPStream(egressAddr)
		defer cleanup()

		var batchSubscriber plumbing.Doppler_BatchSubscribeServer
		Eventually(doppler.BatchSubscribeInput.Stream, 5).Should(Receive(&batchSubscriber))
		go func() {
			response := &plumbing.BatchResponse{
				Payload: buildLogMessage(),
			}

			for {
				err := batchSubscriber.Send(response)
				if err != nil {
					log.Printf("batchSubscriber#Send failed: %s\n", err)
					return
				}
			}
		}()

		envelope, err := egressStream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.GetDeprecatedTags()["origin"].GetText()).To(Equal("some-origin"))
		Expect(dopplerLis.Close()).To(Succeed())
	})

	It("receives container metrics via egress query client", func() {
		doppler, dopplerLis := setupDoppler()
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
		doppler, dopplerLis := setupDoppler()
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
			doppler, dopplerLis := setupDoppler()
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
			doppler, dopplerLis := setupDoppler()
			egressAddr, rlp := setupRLP(dopplerLis, "localhost:0")

			stream, cleanup := setupRLPStream(egressAddr)
			defer cleanup()

			var batchSubscriber plumbing.Doppler_BatchSubscribeServer
			Eventually(doppler.BatchSubscribeInput.Stream, 5).Should(Receive(&batchSubscriber))

			expectedEnvelope := buildLogMessage()
			response := &plumbing.BatchResponse{
				Payload: expectedEnvelope,
			}
			Expect(batchSubscriber.Send(response)).ToNot(HaveOccurred())

			done := make(chan struct{})
			go func() {
				// Wait for envelope to reach buffer
				time.Sleep(250 * time.Millisecond)
				defer close(done)
				rlp.Stop()
			}()

			By("stop reading from the dopplers")
			response2 := &plumbing.BatchResponse{
				Payload: [][]byte{buildContainerMetric()},
			}
			Eventually(func() error { return batchSubscriber.Send(response2) }).Should(HaveOccurred())

			// Currently, the call to Stop() blocks.
			// We need to Recv the message after the call to Stop has been
			// made.
			envelope, err := stream.Recv()
			Expect(err).ToNot(HaveOccurred())

			var e events.Envelope
			err = proto.Unmarshal(expectedEnvelope[0], &e)
			Expect(err).ToNot(HaveOccurred())
			Expect(envelope).To(Equal(conversion.ToV2(&e, false)))

			errs := make(chan error, 100)
			go func() {
				for {
					_, err := stream.Recv()
					errs <- err
				}
			}()
			Eventually(errs).Should(Receive(HaveOccurred()))
			Eventually(done).Should(BeClosed())
			Expect(dopplerLis.Close()).To(Succeed())
		})

	})

	Describe("health endpoint", func() {
		It("returns health metrics", func() {
			doppler, dopplerLis := setupDoppler()
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

func setupDoppler() (*mockDopplerServer, net.Listener) {
	doppler := newMockDopplerServer()

	lis, err := net.Listen("tcp", "localhost:0")
	Expect(err).ToNot(HaveOccurred())

	tlsCredentials, err := plumbing.NewServerCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	Expect(err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))
	plumbing.RegisterDopplerServer(grpcServer, doppler)
	go func() {
		log.Println(grpcServer.Serve(lis))
	}()
	return doppler, lis
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
