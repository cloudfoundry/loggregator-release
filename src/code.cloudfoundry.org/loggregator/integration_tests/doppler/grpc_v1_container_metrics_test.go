package doppler_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container Metrics", func() {
	var (
		appID                   string
		ingressConn, egressConn *grpc.ClientConn
		ingressClient           plumbing.DopplerIngestor_PusherClient
		egressClient            plumbing.DopplerClient
	)

	Context("gRPC V1", func() {
		JustBeforeEach(func() {
			ingressConn, ingressClient = dopplerIngressV1Client("localhost:5678")
			guid, _ := uuid.NewV4()
			appID = guid.String()

			conf := fetchDopplerConfig(pathToConfigFile)
			egressConn, egressClient = connectToGRPC(conf)
		})

		AfterEach(func() {
			ingressConn.Close()
			egressConn.Close()
		})

		It("returns container metrics for an app", func() {
			containerMetric := factories.NewContainerMetric(appID, 0, 1, 2, 3)

			ingressClient.Send(marshalContainerMetric(containerMetric))

			receivedEnvelope := pollForContainerMetric(appID, egressClient)

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ContainerMetric))
			receivedMetric := receivedEnvelope.GetContainerMetric()
			Expect(receivedMetric).To(Equal(containerMetric))
		})

		It("does not receive metrics for different appIds", func() {
			ingressClient.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID+"other", 0, 1, 2, 3),
			))

			goodMetric := factories.NewContainerMetric(appID, 0, 100, 2, 3)
			ingressClient.Send(marshalContainerMetric(goodMetric))

			ingressClient.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID+"other", 1, 1, 2, 3),
			))

			receivedEnvelope := pollForContainerMetric(appID, egressClient)

			Expect(receivedEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(goodMetric))
		})

		It("returns only the latest container metric", func() {
			ingressClient.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID, 0, 10, 2, 3),
			))

			laterMetric := factories.NewContainerMetric(appID, 0, 20, 2, 3)
			ingressClient.Send(marshalContainerMetric(laterMetric))

			receivedEnvelope := pollForContainerMetric(appID, egressClient)

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(laterMetric))
		})
	})
})

func dopplerIngressV1Client(addr string) (*grpc.ClientConn, plumbing.DopplerIngestor_PusherClient) {
	creds, err := plumbing.NewClientCredentials(
		"../fixtures/server.crt",
		"../fixtures/server.key",
		"../fixtures/loggregator-ca.crt",
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(conn)

	var pusherClient plumbing.DopplerIngestor_PusherClient
	f := func() error {
		var err error
		ctx, _ := context.WithTimeout(context.TODO(), 10*time.Second)
		pusherClient, err = client.Pusher(ctx)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())

	return conn, pusherClient
}

func marshalContainerMetric(metric *events.ContainerMetric) *plumbing.EnvelopeData {
	env := &events.Envelope{
		Origin:          proto.String("origin"),
		Timestamp:       proto.Int64(time.Now().UnixNano()),
		EventType:       events.Envelope_ContainerMetric.Enum(),
		ContainerMetric: metric,
	}

	data, err := proto.Marshal(env)
	Expect(err).ToNot(HaveOccurred())

	return &plumbing.EnvelopeData{
		Payload: data,
	}
}

func pollForContainerMetric(appID string, client plumbing.DopplerClient) *events.Envelope {
	var payload [][]byte
	f := func() int {
		ctx, _ := context.WithTimeout(context.TODO(), time.Second)
		resp, err := client.ContainerMetrics(ctx, &plumbing.ContainerMetricsRequest{
			AppID: appID,
		})
		if err != nil {
			return 0
		}

		payload = resp.Payload

		return len(payload)
	}
	Eventually(f).Should(Equal(1))
	env := UnmarshalMessage(payload[0])
	return &env
}
