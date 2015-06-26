package messagewriter_test

import (
	"time"
	"tools/metronbenchmark/messagewriter"

	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChannelLogWriter", func() {
	It("should send a log message to the channel", func() {
		c := make(chan *events.LogMessage, 1)
		clWriter := messagewriter.NewChannelLogWriter(c)
		now := time.Now()
		clWriter.Send(1, now)

		var logMessage *events.LogMessage

		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 1, now)))
		Expect(logMessage.GetMessageType()).To(Equal(events.LogMessage_OUT))
		Expect(logMessage.GetTimestamp()).ToNot(BeZero())
	})

	It("should send a message to the channel with the sequence", func() {
		c := make(chan *events.LogMessage, 2)
		clWriter := messagewriter.NewChannelLogWriter(c)
		now := time.Now()
		roundId := uint(1)
		clWriter.Send(roundId, now)
		clWriter.Send(roundId, now)

		var logMessage *events.LogMessage
		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 1, now)))

		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 2, now)))
	})

	It("should keep track of the total sent messages", func() {
		c := make(chan *events.LogMessage, 2)
		clWriter := messagewriter.NewChannelLogWriter(c)
		now := time.Now()
		roundId := uint(1)
		clWriter.Send(roundId, now)
		clWriter.Send(roundId, now)

		getSent := func() uint {
			return clWriter.GetSentCount(roundId)
		}
		Eventually(getSent).Should(BeEquivalentTo(2))
	})

	It("should keep track of different rounds' sequences independently", func() {
		c := make(chan *events.LogMessage, 5)
		clWriter := messagewriter.NewChannelLogWriter(c)
		now := time.Now()
		roundId1 := uint(1)
		roundId2 := uint(2)
		clWriter.Send(roundId1, now)

		clWriter.Send(roundId2, now)

		clWriter.Send(roundId1, now)

		var logMessage *events.LogMessage
		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 1, now)))

		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(2, 1, now)))

		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 2, now)))
	})

	It("should return empty result for a non-existent round", func() {
		c := make(chan *events.LogMessage, 2)
		clWriter := messagewriter.NewChannelLogWriter(c)
		now := time.Now()
		roundId := uint(1)

		clWriter.Send(roundId, now)

		var logMessage *events.LogMessage
		Expect(c).To(Receive(&logMessage))
		Expect(logMessage.GetMessage()).To(BeEquivalentTo(formatMsgString(1, 1, now)))

		Consistently(c).ShouldNot(Receive())

		Expect(clWriter.GetSentCount(1)).To(BeEquivalentTo(1))
		Expect(clWriter.GetSentCount(2)).To(BeZero())
	})
})
