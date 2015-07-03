package eventmarshaller_test

import (
	"metron/writers/eventmarshaller"
	"metron/writers/mocks"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventMarshaller", func() {
	var (
		marshaller *eventmarshaller.EventMarshaller
		writer     *mocks.MockByteArrayWriter
	)

	BeforeEach(func() {
		writer = &mocks.MockByteArrayWriter{}
		marshaller = eventmarshaller.New(writer, loggertesthelper.Logger())

	})

	It("marshals envelopes into bytes", func() {
		envelope := &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		message, _ := proto.Marshal(envelope)

		marshaller.Write(envelope)

		Expect(writer.Data()).Should(HaveLen(1))
		outputMessage := writer.Data()[0]
		Expect(outputMessage).To(Equal(message))
	})

	Context("metrics", func() {
		It("emits the correct metrics context", func() {
			Expect(marshaller.Emit().Name).To(Equal("eventMarshaller"))
		})

		It("emits a marshal error counter", func() {
			envelope := &events.Envelope{}

			marshaller.Write(envelope)
			testhelpers.EventuallyExpectMetric(marshaller, "marshalErrors", 1)
		})
	})
})
