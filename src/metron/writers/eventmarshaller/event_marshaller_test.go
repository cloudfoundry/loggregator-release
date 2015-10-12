package eventmarshaller_test

import (
	"metron/writers/eventmarshaller"
	"metron/writers/mocks"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventMarshaller", func() {
	var (
		fakeMetricSender *fake.FakeMetricSender
		marshaller       *eventmarshaller.EventMarshaller
		writer           *mocks.MockByteArrayWriter
	)

	BeforeEach(func() {
		fakeMetricSender = fake.NewFakeMetricSender()
		batcher := metricbatcher.New(fakeMetricSender, 1*time.Millisecond)
		metrics.Initialize(fakeMetricSender, batcher)

		writer = &mocks.MockByteArrayWriter{}
		marshaller = eventmarshaller.New(writer, loggertesthelper.Logger())
	})

	Context("with a valid envelope", func() {
		var message []byte

		BeforeEach(func() {
			envelope := &events.Envelope{
				Origin:     proto.String("fake-origin-1"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
			}
			message, _ = proto.Marshal(envelope)

			marshaller.Write(envelope)
		})

		It("marshals envelopes into bytes", func() {
			Expect(writer.Data()).Should(HaveLen(1))
			outputMessage := writer.Data()[0]
			Expect(outputMessage).To(Equal(message))
		})

		It("emits a metric", func() {
			Eventually(func() uint64 { return fakeMetricSender.GetCounter("dropsondeMarshaller.logMessageMarshalled") }).Should(BeEquivalentTo(1))
		})
	})

	Context("invalid envelope", func() {
		It("emits a dropsonde marshal error counter", func() {
			envelope := &events.Envelope{}

			marshaller.Write(envelope)

			Eventually(func() uint64 { return fakeMetricSender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeEquivalentTo(1))
		})
	})
})
