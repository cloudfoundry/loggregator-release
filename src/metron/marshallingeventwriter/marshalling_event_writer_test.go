package marshallingeventwriter_test

import (
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/marshallingeventwriter"
)

var _ = Describe("MarshallingEventWriter", func() {
	var (
		marshaller  *marshallingeventwriter.MarshallingEventWriter
		writer *mockWriter
	)

	BeforeEach(func() {
		writer = &mockWriter{}
		marshaller = marshallingeventwriter.NewMarshallingEventWriter(loggertesthelper.Logger(), writer)

	})

	It("marshals envelopes into bytes", func() {
		envelope := &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		message, _ := proto.Marshal(envelope)

		marshaller.Write(envelope)

		Expect(writer.data).Should(HaveLen(1))
		outputMessage := writer.data[0]
		Expect(outputMessage).To(Equal(message))
	})

	Context("metrics", func() {
		It("emits the correct metrics context", func() {
			Expect(marshaller.Emit().Name).To(Equal("marshallingEventWriter"))
		})

		It("emits a marshal error counter", func() {
			envelope := &events.Envelope{}

			marshaller.Write(envelope)
			testhelpers.EventuallyExpectMetric(marshaller, "marshalErrors", 1)
		})
	})
})

type mockWriter struct {
	data [][]byte
}

func (m *mockWriter) Write(p []byte) (bytesWritten int, err error) {
	m.data = append(m.data, p)
	return len(p), nil
}
