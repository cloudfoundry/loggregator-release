package legacy_message_converter_test

import (
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gogo/protobuf/proto"
	"metron/legacy_message/legacy_message_converter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LegacyMessageConverter", func() {
	var (
		inputChan        chan *logmessage.LogEnvelope
		outputChan       chan *events.Envelope
		runComplete      chan struct{}
		messageConverter *legacy_message_converter.LegacyMessageConverter
	)

	Context("Run", func() {
		BeforeEach(func() {
			inputChan = make(chan *logmessage.LogEnvelope, 10)
			outputChan = make(chan *events.Envelope, 10)
			runComplete = make(chan struct{})
			messageConverter = legacy_message_converter.New(loggertesthelper.Logger())

			go func() {
				messageConverter.Run(inputChan, outputChan)
				close(runComplete)
			}()
		})

		AfterEach(func() {
			close(inputChan)
			Eventually(runComplete).Should(BeClosed())
		})

		It("converts legacy envelopes into dropsonde envelopes", func() {
			legacyEnvelope := &logmessage.LogEnvelope{
				RoutingKey: proto.String("fake-routing-key"),
				Signature:  []byte{1, 2, 3},
				LogMessage: &logmessage.LogMessage{
					Message:     []byte{4, 5, 6},
					MessageType: logmessage.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(123),
					AppId:       proto.String("fake-app-id"),
					SourceId:    proto.String("fake-source-id"),
					SourceName:  proto.String("fake-source-name"),
				},
			}

			inputChan <- legacyEnvelope

			Eventually(outputChan).Should(Receive(Equal(&events.Envelope{
				Origin:    proto.String(legacy_message_converter.LEGACY_DROPSONDE_ORIGIN),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:        []byte{4, 5, 6},
					MessageType:    events.LogMessage_OUT.Enum(),
					Timestamp:      proto.Int64(123),
					AppId:          proto.String("fake-app-id"),
					SourceType:     proto.String("fake-source-name"),
					SourceInstance: proto.String("fake-source-id"),
				},
			})))
		})
	})
})
