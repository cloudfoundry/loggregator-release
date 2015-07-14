package synchronizedwriter_test

import (
	. "metron/writers/synchronizedwriter"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/writers/mocks"
	"strconv"
	"sync"
)

var _ = Describe("Synchronizedwriter", func() {

	var synchronizedWriter *SynchronizedWriter
	var mockEnvelopeWriter *mocks.MockEnvelopeWriter

	BeforeEach(func() {
		mockEnvelopeWriter = &mocks.MockEnvelopeWriter{}
		synchronizedWriter = New(mockEnvelopeWriter)
	})

	Context("when writer is called with a message", func() {
		It("can write the message", func() {
			eventEnvelope := &events.Envelope{}
			Expect(mockEnvelopeWriter.Events).To(HaveLen(0))
			synchronizedWriter.Write(eventEnvelope)
			Expect(mockEnvelopeWriter.Events).To(HaveLen(1))
		})
	})

	Context("when writer is called multiple times with different messages", func() {
		It("can write all the messages", func() {
			eventEnvelopeOne := createCounterMessage("one", "fake-origin")
			eventEnvelopeTwo := createCounterMessage("two", "fake-origin")
			Expect(mockEnvelopeWriter.Events).To(HaveLen(0))
			synchronizedWriter.Write(eventEnvelopeOne)
			Expect(mockEnvelopeWriter.Events).To(HaveLen(1))
			synchronizedWriter.Write(eventEnvelopeTwo)
			Expect(mockEnvelopeWriter.Events).To(HaveLen(2))
			Expect(mockEnvelopeWriter.Events).To(ConsistOf(eventEnvelopeOne, eventEnvelopeTwo))
		})
	})

	Context("when a writer is called from multiple goroutines", func() {
		It("can write all the messages", func() {
			const iterations = 100
			var wg sync.WaitGroup
			writeEvents := func() {
				for i := 0; i < iterations; i++ {
					envelope := createCounterMessage(strconv.Itoa(i), "fake-origin")
					synchronizedWriter.Write(envelope)
				}
				wg.Done()
			}

			const numGoRoutines = 5
			for i := 0; i < numGoRoutines; i++ {
				wg.Add(1)
				go writeEvents()
			}

			wg.Wait()
			Expect(mockEnvelopeWriter.Events).To(HaveLen(iterations * numGoRoutines))
		})
	})
})

func createCounterMessage(name string, origin string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String(name),
			Delta: proto.Uint64(4),
		},
	}
}
