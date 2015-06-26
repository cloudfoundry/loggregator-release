package messagereader_test

import (
	. "tools/metronbenchmark/messagereader"

	"time"
	"tools/metronbenchmark/messagewriter"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Messagereader", func() {
	It("should read from a channel and update data structure", func() {
		c := make(chan *events.LogMessage)
		writer := messagewriter.NewChannelLogWriter(c)
		reader := NewMessageReader(NewChannelReader(c))
		go reader.Start()

		writer.Send(1, time.Now())
		writer.Send(1, time.Now())

		getReceived := func() uint {
			v := reader.GetRoundResult(1)
			return v.Received
		}

		Eventually(getReceived).Should(BeEquivalentTo(2))
	})

	It("should return empty result for a non-existent round", func() {
		c := make(chan *events.LogMessage)
		reader := NewMessageReader(NewChannelReader(c))
		Expect(reader.GetRoundResult(1)).To(BeNil())
	})

	It("should return max latency for a given round", func() {
		c := make(chan *events.LogMessage)
		writer := messagewriter.NewChannelLogWriter(c)
		reader := NewMessageReader(NewChannelReader(c))
		go reader.Start()
		writer.Send(1, time.Now().Add(-time.Minute))
		writer.Send(1, time.Now())

		getReceived := func() uint {
			v := reader.GetRoundResult(1)
			return v.Received
		}

		Eventually(getReceived).Should(BeEquivalentTo(2))
		Expect(reader.GetRoundResult(1).Latency).Should(BeNumerically(">=", time.Minute))

	})
})
