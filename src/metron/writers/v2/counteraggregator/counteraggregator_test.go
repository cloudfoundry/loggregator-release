package counteraggregator_test

import (
	"metron/writers/v2/counteraggregator"
	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counteraggregator", func() {
	It("forwards non-counter enevelopes", func() {
		mockWriter := newMockWriter()
		close(mockWriter.WriteOutput.Err)

		logEnvelope := &v2.Envelope{
			Message: &v2.Envelope_Log{
				Log: &v2.Log{
					Payload: []byte("some-message"),
					Type:    v2.Log_OUT,
				},
			},
		}

		aggregator := counteraggregator.New(mockWriter)
		aggregator.Write(logEnvelope)

		Expect(mockWriter.WriteInput.Msg).To(Receive(Equal(logEnvelope)))
	})

	It("calculates totals for same counter envelopes", func() {
		mockWriter := newMockWriter()
		close(mockWriter.WriteOutput.Err)

		aggregator := counteraggregator.New(mockWriter)
		aggregator.Write(buildCounterEnvelope(10, "name-1", "origin-1"))
		aggregator.Write(buildCounterEnvelope(15, "name-1", "origin-1"))

		var receivedEnvelope *v2.Envelope
		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(10)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(25)))
	})

	It("calculates totals separately for counter envelopes with unique names", func() {
		mockWriter := newMockWriter()
		close(mockWriter.WriteOutput.Err)

		aggregator := counteraggregator.New(mockWriter)
		aggregator.Write(buildCounterEnvelope(10, "name-1", "origin-1"))
		aggregator.Write(buildCounterEnvelope(15, "name-2", "origin-1"))
		aggregator.Write(buildCounterEnvelope(20, "name-3", "origin-1"))

		var receivedEnvelope *v2.Envelope
		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(10)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(15)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(20)))
	})

	It("calculates totals separately for counter envelopes with same name but unique tags", func() {
		mockWriter := newMockWriter()
		close(mockWriter.WriteOutput.Err)

		aggregator := counteraggregator.New(mockWriter)
		aggregator.Write(buildCounterEnvelope(10, "name-1", "origin-1"))
		aggregator.Write(buildCounterEnvelope(15, "name-1", "origin-1"))
		aggregator.Write(buildCounterEnvelope(20, "name-1", "origin-2"))

		var receivedEnvelope *v2.Envelope
		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(10)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(25)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(20)))
	})

	It("calculations are unaffected for counter envelopes with total set", func() {
		mockWriter := newMockWriter()
		close(mockWriter.WriteOutput.Err)

		aggregator := counteraggregator.New(mockWriter)
		aggregator.Write(buildCounterEnvelope(10, "name-1", "origin-1"))
		aggregator.Write(buildCounterEnvelopeWithTotal(5000, "name-1", "origin-1"))

		var receivedEnvelope *v2.Envelope
		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(10)))

		Expect(mockWriter.WriteInput.Msg).To(Receive(&receivedEnvelope))
		Expect(receivedEnvelope.GetCounter().GetTotal()).To(Equal(uint64(10)))
	})
})

func buildCounterEnvelope(delta uint64, name, origin string) *v2.Envelope {
	return &v2.Envelope{
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: name,
				Value: &v2.Counter_Delta{
					Delta: delta,
				},
			},
		},
		Tags: map[string]*v2.Value{
			"origin": {Data: &v2.Value_Text{origin}},
		},
	}
}

func buildCounterEnvelopeWithTotal(total uint64, name, origin string) *v2.Envelope {
	return &v2.Envelope{
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: name,
				Value: &v2.Counter_Total{
					Total: total,
				},
			},
		},
		Tags: map[string]*v2.Value{
			"origin": {Data: &v2.Value_Text{origin}},
		},
	}
}
