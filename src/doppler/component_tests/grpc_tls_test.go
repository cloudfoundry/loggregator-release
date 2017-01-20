package component_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"plumbing"
	v2 "plumbing/v2"
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

	It("supports v2 api", func() {
		hostPort, cleanup := setupDopplerEnv()
		defer cleanup()
		subscriber := setupSubscriber(hostPort)
		sender := setupV2Ingestor(hostPort)

		v2e, _ := buildV2ContainerMetric()
		v1e, _ := buildV1ContainerMetric()
		v1e.Timestamp = proto.Int64(v2e.Timestamp)

		Consistently(func() error {
			return sender.Send(v2e)
		}, 5).Should(Succeed())

		f := func() *events.Envelope {
			resp, err := subscriber.Recv()
			Expect(err).ToNot(HaveOccurred())

			v1e := &events.Envelope{}
			Expect(v1e.Unmarshal(resp.Payload)).To(Succeed())
			return v1e
		}
		Eventually(f).Should(Equal(v1e))
	})
})

func setupDopplerEnv() (string, func()) {
	etcdCleanup, etcdURI := testservers.StartTestEtcd()

	By("listen for doppler writes into metron")
	udpAddr, err := net.ResolveUDPAddr("udp", ":0")
	Expect(err).ToNot(HaveOccurred())
	udpLn, err := net.ListenUDP("udp", udpAddr)
	Expect(err).ToNot(HaveOccurred())

	dopplerCleanup, _, dopplerPort := testservers.StartDoppler(
		testservers.BuildDopplerConfig(etcdURI, udpAddr.Port),
	)
	hostPort := fmt.Sprintf("localhost:%d", dopplerPort)
	return hostPort, func() {
		etcdCleanup()
		dopplerCleanup()
		udpLn.Close()
	}
}

func setupV1Ingestor(hostPort string) plumbing.DopplerIngestor_PusherClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		testservers.MetronCertPath(),
		testservers.MetronKeyPath(),
		testservers.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(hostPort, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(c)

	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	pusher, err := client.Pusher(ctx)
	Expect(err).ToNot(HaveOccurred())

	return pusher
}

func setupV2Ingestor(hostPort string) v2.DopplerIngress_SenderClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		testservers.MetronCertPath(),
		testservers.MetronKeyPath(),
		testservers.CAFilePath(),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())
	transportCreds := credentials.NewTLS(tlsConfig)
	c, err := grpc.Dial(hostPort, grpc.WithTransportCredentials(transportCreds))
	Expect(err).ToNot(HaveOccurred())
	client := v2.NewDopplerIngressClient(c)

	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	sender, err := client.Sender(ctx)
	Expect(err).ToNot(HaveOccurred())

	return sender
}

func setupSubscriber(hostPort string) plumbing.Doppler_SubscribeClient {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		testservers.TrafficControllerCertPath(),
		testservers.TrafficControllerKeyPath(),
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
		Origin:     proto.String("doppler"),
		EventType:  events.Envelope_ContainerMetric.Enum(),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		Deployment: proto.String(""),
		Job:        proto.String(""),
		Index:      proto.String(""),
		Ip:         proto.String(""),
		Tags:       map[string]string{"origin": "doppler"},
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("some-app"),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(1),
			MemoryBytes:   proto.Uint64(1),
			DiskBytes:     proto.Uint64(1),
			/// todo tes something here
			MemoryBytesQuota: proto.Uint64(1),
			DiskBytesQuota:   proto.Uint64(1),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func buildV2ContainerMetric() (*v2.Envelope, []byte) {
	envelope := &v2.Envelope{
		SourceUuid: "some-app",
		Timestamp:  time.Now().UnixNano(),
		Message: &v2.Envelope_Gauge{
			Gauge: &v2.Gauge{
				Metrics: map[string]*v2.GaugeValue{
					"instance_index": {Value: 1},
					"cpu":            {Value: 1},
					"memory":         {Value: 1},
					"disk":           {Value: 1},
					"memory_quota":   {Value: 1},
					"disk_quota":     {Value: 1},
				},
			},
		},
		Tags: map[string]*v2.Value{
			"origin": {Data: &v2.Value_Text{"doppler"}},
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}
