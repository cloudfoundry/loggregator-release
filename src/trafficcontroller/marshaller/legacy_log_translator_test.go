package marshaller_test

import (
	"trafficcontroller/marshaller"

	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TranslateDropsondeToLegacyLogMessage", func() {
	It("converts well-formed Dropsonde log messages to legacy format", func() {
		message := []byte("message")
		envelope := &events.Envelope{
			Origin:    proto.String("origin"),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Message:        message,
				MessageType:    events.LogMessage_OUT.Enum(),
				Timestamp:      proto.Int64(1234),
				AppId:          proto.String("AppId"),
				SourceType:     proto.String("SRC"),
				SourceInstance: proto.String("0"),
			},
		}

		bytes, _ := proto.Marshal(envelope)
		msg, err := marshaller.TranslateDropsondeToLegacyLogMessage(bytes)

		Expect(err).To(BeNil())

		var legacyMessage logmessage.LogMessage
		proto.Unmarshal(msg, &legacyMessage)
		Expect(legacyMessage.GetMessage()).To(Equal(message))
		Expect(legacyMessage.GetAppId()).To(Equal("AppId"))
		Expect(legacyMessage.GetSourceName()).To(Equal("SRC"))
		Expect(legacyMessage.GetSourceId()).To(Equal("0"))
		Expect(legacyMessage.GetTimestamp()).To(BeNumerically("==", 1234))
	})

	It("returns an error if it fails to unmarshall the envelope", func() {
		junk := []byte{1, 2, 3}

		msg, err := marshaller.TranslateDropsondeToLegacyLogMessage(junk)

		Expect(msg).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("Unable to unmarshal"))
	})

	It("returns an error if the envelope is of type other than LogMessage", func() {
		envelope := &events.Envelope{
			Origin:    proto.String("origin"),
			EventType: events.Envelope_CounterEvent.Enum(),
		}
		bytes, _ := proto.Marshal(envelope)

		msg, err := marshaller.TranslateDropsondeToLegacyLogMessage(bytes)

		Expect(msg).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("Envelope contained CounterEvent"))
	})

	It("returns an error if the envelope doesn't contain a LogMessage", func() {
		envelope := &events.Envelope{
			Origin:    proto.String("origin"),
			EventType: events.Envelope_LogMessage.Enum(),
		}
		bytes, _ := proto.Marshal(envelope)

		msg, err := marshaller.TranslateDropsondeToLegacyLogMessage(bytes)

		Expect(msg).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("Envelope's LogMessage was nil"))
	})
})
