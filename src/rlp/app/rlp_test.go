package app_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"metricemitter/testhelper"
	"net"
	"net/http"
	"plumbing"
	v2 "plumbing/v2"
	app "rlp/app"
	"testservers"
	"time"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Start", func() {
	It("receive messages via egress client", func() {
		doppler, dopplerLis := setupDoppler()
		defer dopplerLis.Close()

		egressLis := setupRLP(dopplerLis, "localhost:0")

		egressStream, cleanup := setupRLPClient(egressLis)
		defer cleanup()

		var subscriber plumbing.Doppler_SubscribeServer
		Eventually(doppler.SubscribeInput.Stream, 5).Should(Receive(&subscriber))
		go func() {
			response := &plumbing.Response{
				Payload: buildLogMessage(),
			}

			for {
				err := subscriber.Send(response)
				if err != nil {
					log.Printf("subscriber#Send failed: %s\n", err)
					return
				}
			}
		}()

		envelope, err := egressStream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.GetTags()["origin"].GetText()).To(Equal("some-origin"))
	})

	It("receives container metrics via egress query client", func() {
		doppler, dopplerLis := setupDoppler()
		defer dopplerLis.Close()
		doppler.ContainerMetricsOutput.Err <- nil
		doppler.ContainerMetricsOutput.Resp <- &plumbing.ContainerMetricsResponse{
			Payload: [][]byte{buildContainerMetric()},
		}

		egressLis := setupRLP(dopplerLis, "localhost:0")
		egressClient, cleanup := setupRLPQueryClient(egressLis)
		defer cleanup()

		var resp *v2.QueryResponse
		f := func() error {
			var err error
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			resp, err = egressClient.ContainerMetrics(ctx, &v2.ContainerMetricRequest{
				SourceId: "some-app",
			})

			return err
		}
		Eventually(f).Should(Succeed())

		Expect(resp.Envelopes).To(HaveLen(1))
	})

	Describe("health endpoint", func() {
		It("returns health metrics", func() {
			doppler, dopplerLis := setupDoppler()
			defer dopplerLis.Close()
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

			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("subscriptionCount"))
		})
	})
})

func buildLogMessage() []byte {
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
	return b
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

	tlsCredentials, err := plumbing.NewCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))
	plumbing.RegisterDopplerServer(grpcServer, doppler)
	go grpcServer.Serve(lis)
	return doppler, lis
}

func setupRLP(dopplerLis net.Listener, healthAddr string) net.Listener {
	egressLis, err := net.Listen("tcp", "localhost:0")
	egressLis.Close()
	Expect(err).ToNot(HaveOccurred())

	ingressTLSCredentials, err := plumbing.NewCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	egressTLSCredentials, err := plumbing.NewCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	egressPort := egressLis.Addr().(*net.TCPAddr).Port
	rlp := app.NewRLP(
		testhelper.NewMetricClient(),
		app.WithEgressPort(egressPort),
		app.WithIngressAddrs([]string{dopplerLis.Addr().String()}),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(ingressTLSCredentials)),
		app.WithEgressServerOptions(grpc.Creds(egressTLSCredentials)),
		app.WithHealthAddr(healthAddr),
	)
	go rlp.Start()
	return egressLis
}

func setupRLPClient(egressLis net.Listener) (v2.Egress_ReceiverClient, func()) {
	ingressTLSCredentials, err := plumbing.NewCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(
		egressLis.Addr().String(),
		grpc.WithTransportCredentials(ingressTLSCredentials),
	)
	Expect(err).ToNot(HaveOccurred())

	egressClient := v2.NewEgressClient(conn)

	var egressStream v2.Egress_ReceiverClient
	Eventually(func() error {
		egressStream, err = egressClient.Receiver(context.Background(), &v2.EgressRequest{})
		return err
	}, 5).ShouldNot(HaveOccurred())

	return egressStream, func() {
		conn.Close()
	}
}

func setupRLPQueryClient(egressLis net.Listener) (v2.EgressQueryClient, func()) {
	ingressTLSCredentials, err := plumbing.NewCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
		"reverselogproxy",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(
		egressLis.Addr().String(),
		grpc.WithTransportCredentials(ingressTLSCredentials),
	)
	Expect(err).ToNot(HaveOccurred())

	egressClient := v2.NewEgressQueryClient(conn)

	return egressClient, func() {
		conn.Close()
	}
}
