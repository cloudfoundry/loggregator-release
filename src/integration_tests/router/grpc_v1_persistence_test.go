package router_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/integration_tests/fakes"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Persistence", func() {
	Describe("Recent Logs", func() {
		It("receives recent log messages", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			logMessage := NewLogMessage(events.LogMessage_OUT, "msg 1", "some-test-app-id", "APP")
			marshalledLogMessage := marshalLogMessage(logMessage)

			err := ingressClient.Send(marshalledLogMessage)
			Expect(err).ToNot(HaveOccurred())

			receivedMessage := pollForRecentLogs("some-test-app-id", egressClient).GetLogMessage()
			Expect(receivedMessage.GetAppId()).To(Equal("some-test-app-id"))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("only receives messages for the specified appId", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			logMessage0 := NewLogMessage(events.LogMessage_OUT, "msg 1", "some-test-app-id", "APP")
			marshalledLogMessage0 := marshalLogMessage(logMessage0)
			logMessage1 := NewLogMessage(events.LogMessage_OUT, "msg 2", "some-other-app-id", "APP")
			marshalledLogMessage1 := marshalLogMessage(logMessage1)

			err := ingressClient.Send(marshalledLogMessage0)
			Expect(err).ToNot(HaveOccurred())
			err = ingressClient.Send(marshalledLogMessage1)
			Expect(err).ToNot(HaveOccurred())

			receivedMessage := pollForRecentLogs("some-test-app-id", egressClient).GetLogMessage()
			Expect(receivedMessage.GetAppId()).To(Equal("some-test-app-id"))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("only receives the most recent logs", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			for i := 0; i < 15; i++ {
				logMessage := NewLogMessage(events.LogMessage_OUT, strconv.Itoa(i), "some-test-app-id", "APP")
				err := ingressClient.Send(marshalLogMessage(logMessage))
				Expect(err).ToNot(HaveOccurred())
			}

			Eventually(func() []string {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				resp, err := egressClient.RecentLogs(ctx, &plumbing.RecentLogsRequest{
					AppID: "some-test-app-id",
				})
				if err != nil {
					return []string{}
				}

				var messages []string
				for _, messageBytes := range resp.Payload {
					receivedMessage := decodeProtoBufLogMessage(messageBytes)
					messages = append(messages, string(receivedMessage.GetMessage()))
				}

				return messages
			}).Should(Equal([]string{"5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}))
		})
	})
})

func marshalLogMessage(log *events.LogMessage) *plumbing.EnvelopeData {
	env := &events.Envelope{
		Origin:     proto.String("origin"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		EventType:  events.Envelope_LogMessage.Enum(),
		LogMessage: log,
	}

	data, err := proto.Marshal(env)
	Expect(err).ToNot(HaveOccurred())

	return &plumbing.EnvelopeData{
		Payload: data,
	}
}

func pollForRecentLogs(appID string, client plumbing.DopplerClient) *events.Envelope {
	var payload [][]byte
	f := func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := client.RecentLogs(ctx, &plumbing.RecentLogsRequest{
			AppID: appID,
		})
		if err != nil {
			return 0
		}

		payload = resp.Payload

		return len(payload)
	}
	Eventually(f).Should(Equal(1))
	env := unmarshalMessage(payload[0])
	return &env
}
