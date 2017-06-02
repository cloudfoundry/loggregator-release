package doppler_test

import (
	"plumbing"
	"strconv"
	"time"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	uuid "github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recent Logs", func() {
	var (
		appID         string
		ingressConn   *grpc.ClientConn
		ingressClient plumbing.DopplerIngestor_PusherClient
	)

	Context("gRPC v1", func() {
		JustBeforeEach(func() {
			ingressConn, ingressClient = dopplerIngressV1Client("localhost:5678")
			guid, _ := uuid.NewV4()
			appID = guid.String()
		})

		AfterEach(func() {
			ingressConn.Close()
		})

		It("receives recent log messages", func() {
			ingressClient.Send(marshalLogMessage(
				factories.NewLogMessage(events.LogMessage_OUT, "msg 1", appID, "APP"),
			))

			returnedMessages := make([][]byte, 1)
			Eventually(func() [][]byte {
				returnedMessages = retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(1))

			receivedMessage := DecodeProtoBufLogMessage(returnedMessages[0])

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("only receives messages for the specified appId", func() {
			logMessage := factories.NewLogMessage(events.LogMessage_OUT, "msg 1", appID, "APP")
			ingressClient.Send(marshalLogMessage(logMessage))

			logMessage = factories.NewLogMessage(events.LogMessage_OUT, "msg 2", "otherId", "APP")
			ingressClient.Send(marshalLogMessage(logMessage))

			returnedMessages := make([][]byte, 1)
			Eventually(func() [][]byte {
				returnedMessages = retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(1))

			receivedMessage := DecodeProtoBufLogMessage(returnedMessages[0])

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("does not receive non-log messages", func() {
			ingressClient.Send(marshalContainerMetric(
				factories.NewContainerMetric(appID, 0, 10, 10, 10),
			))

			Consistently(func() [][]byte {
				returnedMessages := retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(0))
		})

		It("only receives the most recent logs", func() {
			for i := 0; i < 15; i++ {
				logMessage := factories.NewLogMessage(events.LogMessage_OUT, strconv.Itoa(i), appID, "APP")
				ingressClient.Send(marshalLogMessage(logMessage))
			}

			Eventually(func() []string {
				returnedMessages := retreiveRecentMessages(appID)
				var messages []string

				for _, messageBytes := range returnedMessages {
					receivedMessage := DecodeProtoBufLogMessage(messageBytes)
					messages = append(messages, string(receivedMessage.GetMessage()))
				}

				return messages
			}).Should(Equal([]string{"5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}))
		})
	})
})

func retreiveRecentMessages(appID string) [][]byte {
	rChan := make(chan []byte, 10)

	ws, _ := AddWSSink(rChan, "4567", "/apps/"+appID+"/recentlogs")
	defer ws.Close()

	returnedMessages := make([][]byte, 0)
	for message := range rChan {
		returnedMessages = append(returnedMessages, message)
	}

	return returnedMessages
}

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
