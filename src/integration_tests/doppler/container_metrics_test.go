package doppler_test

import (
	"context"
	"plumbing"
	"time"

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
		receivedChan chan []byte
		appID        string
		conn         *grpc.ClientConn
		client       plumbing.DopplerIngestor_PusherClient
	)

	Context("gRPC V1", func() {
		JustBeforeEach(func() {
			conn, client = dopplerIngressV1Client("localhost:5678")
			guid, _ := uuid.NewV4()
			appID = guid.String()
		})

		AfterEach(func() {
			conn.Close()
		})

		It("returns container metrics for an app", func() {
			containerMetric := factories.NewContainerMetric(appID, 0, 1, 2, 3)

			client.Send(marshalContainerMetric(containerMetric))

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ContainerMetric))
			receivedMetric := receivedEnvelope.GetContainerMetric()
			Expect(receivedMetric).To(Equal(containerMetric))
		})

		It("does not receive metrics for different appIds", func() {
			client.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID+"other", 0, 1, 2, 3),
			))

			goodMetric := factories.NewContainerMetric(appID, 0, 100, 2, 3)
			client.Send(marshalContainerMetric(goodMetric))

			client.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID+"other", 1, 1, 2, 3),
			))

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
			Eventually(receivedChan).Should(BeClosed())

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(goodMetric))
		})

		XIt("returns metrics for all instances of the app", func() {
			client.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID, 0, 1, 2, 3),
			))
			client.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID, 1, 1, 2, 3),
			))

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var firstReceivedMessageBytes []byte
			var secondReceivedMessageBytes []byte

			Eventually(receivedChan).Should(Receive(&firstReceivedMessageBytes))
			Eventually(receivedChan).Should(Receive(&secondReceivedMessageBytes))

			firstEnvelope := UnmarshalMessage(firstReceivedMessageBytes)
			secondEnvelope := UnmarshalMessage(secondReceivedMessageBytes)

			Expect(firstEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(secondEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(firstEnvelope.GetContainerMetric()).NotTo(Equal(secondEnvelope.GetContainerMetric()))
		})

		It("returns only the latest container metric", func() {
			client.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID, 0, 10, 2, 3),
			))

			laterMetric := factories.NewContainerMetric(appID, 0, 20, 2, 3)
			client.Send(marshalContainerMetric(laterMetric))

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(laterMetric))
		})
	})
})

func dopplerIngressV1Client(addr string) (*grpc.ClientConn, plumbing.DopplerIngestor_PusherClient) {
	creds, err := plumbing.NewCredentials(
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
