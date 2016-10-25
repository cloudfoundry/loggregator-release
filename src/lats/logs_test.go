package lats_test

import (
	"crypto/tls"
	"lats/helpers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	It("gets through recent logs", func() {
		env := createLogEnvelope("I AM A BANANA!", "foo")
		helpers.EmitToMetron(env)

		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

		getRecentLogs := func() []*events.LogMessage {
			envelopes, err := consumer.RecentLogs("foo", "")
			Expect(err).NotTo(HaveOccurred())
			return envelopes
		}

		Eventually(getRecentLogs).Should(ContainElement(env.LogMessage))
	})

	It("sends log messages for a specific app through the stream endpoint", func() {
		msgChan, errorChan := helpers.ConnectToStream("foo")

		helpers.EmitToMetron(createLogEnvelope("Stream message", "bar"))
		helpers.EmitToMetron(createLogEnvelope("Stream message", "foo"))

		receivedEnvelope, err := helpers.FindMatchingEnvelopeByID("foo", msgChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(receivedEnvelope).NotTo(BeNil())
		Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_LogMessage))

		event := receivedEnvelope.GetLogMessage()
		Expect(envelope_extensions.GetAppId(receivedEnvelope)).To(Equal("foo"))
		Expect(string(event.GetMessage())).To(Equal("Stream message"))
		Expect(event.GetMessageType().String()).To(Equal(events.LogMessage_OUT.Enum().String()))
		Expect(event.GetTimestamp()).ToNot(BeZero())

		Expect(errorChan).To(BeEmpty())
	})
})

func createLogEnvelope(message, appId string) *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_LogMessage.Enum(),
		Origin:    proto.String(helpers.ORIGIN_NAME),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte(message),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appId),
		},
	}
}
