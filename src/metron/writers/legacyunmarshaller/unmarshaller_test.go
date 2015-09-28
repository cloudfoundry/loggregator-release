package legacyunmarshaller_test

import (
	"metron/writers/legacyunmarshaller"
	"metron/writers/mocks"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("LegacyUnmarshaller", func() {
	var (
		unmarshaller *legacyunmarshaller.LegacyUnmarshaller
		writer       mocks.MockEnvelopeWriter
		fakeSender   *fake.FakeMetricSender
	)

	Context("Run", func() {
		BeforeEach(func() {
			writer = mocks.MockEnvelopeWriter{}
			unmarshaller = legacyunmarshaller.New(&writer, loggertesthelper.Logger())
		})

		It("unmarshals bytes on channel into envelopes", func() {
			envelope := &logmessage.LogEnvelope{
				RoutingKey: proto.String("fake-routing-key"),
				Signature:  []byte{1, 2, 3},
				LogMessage: &logmessage.LogMessage{
					Message:     []byte{4, 5, 6},
					MessageType: logmessage.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(123),
					AppId:       proto.String("fake-app-id"),
				},
			}
			message, _ := proto.Marshal(envelope)

			unmarshaller.Write(message)
			Expect(writer.Events).To(ConsistOf(&events.Envelope{
				Origin:    proto.String("legacy"),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:     []byte{4, 5, 6},
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(123),
					AppId:       proto.String("fake-app-id"),
				},
			}))
		})

		It("does not put an envelope on the output channel if there is unmarshal error", func() {
			unmarshaller.Write([]byte{1, 2, 3})
			Expect(writer.Events).To(BeEmpty())
		})
	})

	Context("metrics", func() {
		BeforeEach(func() {
			unmarshaller = legacyunmarshaller.New(&writer, loggertesthelper.Logger())
			fakeSender = fake.NewFakeMetricSender()
			batcher := metricbatcher.New(fakeSender, time.Millisecond)
			metrics.Initialize(fakeSender, batcher)
		})

		It("emits an unmarshal error counter", func() {
			unmarshaller.Write([]byte{1, 2, 3})
			Eventually(func() uint64 { return fakeSender.GetCounter("legacyUnmarshaller.unmarshalErrors") }).Should(BeEquivalentTo(1))
		})
	})
})
