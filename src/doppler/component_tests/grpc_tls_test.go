package component_test

import (
	// . "doppler/component_tests"
	"context"
	"fmt"
	"net"
	"plumbing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"testservers"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("gRPC TLS", func() {
	It("supports v1 api", func() {
		hostPort, cleanup := setupDopplerEnv()
		defer cleanup()
		subscriber := setupSubscriber(hostPort)
		sender := setupV1Ingestor(hostPort)

		_, data := buildV1ContainerMetric()
		message := &plumbing.EnvelopeData{data}
		Consistently(func() error {
			return sender.Send(message)
		}, 5).Should(Succeed())

		f := func() []byte {
			resp, err := subscriber.Recv()
			Expect(err).ToNot(HaveOccurred())
			return resp.Payload
		}
		Eventually(f).Should(Equal(data))
	})
})

func setupDopplerEnv() (string, func()) {
	etcdCleanup, etcdURI := testservers.StartTestEtcd()

	By("listen for doppler writes into metron")
	// TODO: get random port from kernel....
	udpAddr, err := net.ResolveUDPAddr("udp", ":12345")
	Expect(err).ToNot(HaveOccurred())
	_, err = net.ListenUDP("udp", udpAddr)

	dopplerCleanup, _, dopplerPort := testservers.StartDoppler(
		testservers.BuildDopplerConfig(etcdURI, 12345),
	)
	hostPort := fmt.Sprintf("localhost:%d", dopplerPort)
	return hostPort, func() {
		etcdCleanup()
		dopplerCleanup()
	}
}

func setupV1Ingestor(hostPort string) plumbing.DopplerIngestor_PusherClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		testservers.ClientCertFilePath(),
		testservers.ClientKeyFilePath(),
		testservers.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(hostPort, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(c)

	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(3*time.Second))
	pusher, err := client.Pusher(ctx)
	Expect(err).ToNot(HaveOccurred())

	return pusher
}

func setupSubscriber(hostPort string) plumbing.Doppler_SubscribeClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		testservers.ClientCertFilePath(),
		testservers.ClientKeyFilePath(),
		testservers.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(hostPort, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerClient(c)

	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	subscriber, err := client.Subscribe(ctx, &plumbing.SubscriptionRequest{
		ShardID: "test-shard",
	})
	Expect(err).ToNot(HaveOccurred())

	return subscriber
}

func buildV1ContainerMetric() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("some-app"),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}
