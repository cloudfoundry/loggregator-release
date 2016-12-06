package component_test

import (
	// . "doppler/component_tests"
	"context"
	"fmt"
	"integration_tests"
	"net"
	"plumbing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// have fake metron that sends only on grpc tls
// ?? maybe need fake etcd if it fails
// connect to grpc stream endpoints and get data out

func setupDopplerEnv() (string, func()) {
	etcdCleanup, etcdURI := integration_tests.SetupEtcd()

	// todo: get random port from kernel....
	udpAddr, err := net.ResolveUDPAddr("udp", ":12345")
	Expect(err).ToNot(HaveOccurred())

	_, err = net.ListenUDP("udp", udpAddr)
	dopplerCleanup, _, dopplerPort := integration_tests.SetupDoppler(etcdURI, 12345)
	uri := fmt.Sprintf("localhost:%d", dopplerPort)
	return uri, func() {
		etcdCleanup()
		dopplerCleanup()
	}
}

func setupIngestor(uri string) plumbing.DopplerIngestor_PusherClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		integration_tests.ClientCertFilePath(),
		integration_tests.ClientKeyFilePath(),
		integration_tests.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(uri, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(c)

	pusher, err := client.Pusher(context.Background())
	Expect(err).ToNot(HaveOccurred())

	return pusher
}

func setupSubscriber(uri string) plumbing.Doppler_SubscribeClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		integration_tests.ClientCertFilePath(),
		integration_tests.ClientKeyFilePath(),
		integration_tests.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(uri, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerClient(c)

	subscriber, err := client.Subscribe(context.Background(), &plumbing.SubscriptionRequest{
		ShardID: "test-shard",
	})
	Expect(err).ToNot(HaveOccurred())

	return subscriber
}

var _ = Describe("gRPC TLS", func() {
	It("works", func() {
		uri, cleanup := setupDopplerEnv()
		defer cleanup()
		subscriber := setupSubscriber(uri)
		pusher := setupIngestor(uri)

		_, data := buildContainerMetric()
		message := &plumbing.EnvelopeData{data}
		Consistently(func() error {
			return pusher.Send(message)
		}, 5).Should(Succeed())

		f := func() []byte {
			resp, err := subscriber.Recv()
			Expect(err).ToNot(HaveOccurred())
			return resp.Payload
		}
		Eventually(f).Should(Equal(data))
	})
})

func buildContainerMetric() (*events.Envelope, []byte) {
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
