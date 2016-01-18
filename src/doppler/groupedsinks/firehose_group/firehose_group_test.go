package firehose_group_test

import (
	"doppler/sinks"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"

	"doppler/groupedsinks/firehose_group"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeSink struct {
	sinkId string
	appId  string
}

func (f *fakeSink) AppID() string {
	return f.appId
}

func (f *fakeSink) Run(<-chan *events.Envelope) {
}

func (f *fakeSink) Identifier() string {
	return f.sinkId
}

func (f *fakeSink) ShouldReceiveErrors() bool {
	return false
}

func (f *fakeSink) GetInstrumentationMetric() sinks.Metric {
	return sinks.Metric{}
}

func (f *fakeSink) UpdateDroppedMessageCount(messageCount int64) {}

var _ = Describe("FirehoseGroup", func() {
	It("sends message to all registered sinks", func() {
		receiveChan1 := make(chan *events.Envelope, 10)
		receiveChan2 := make(chan *events.Envelope, 10)

		sink1 := fakeSink{appId: "firehose-a", sinkId: "sink-a"}
		sink2 := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

		group := firehose_group.NewFirehoseGroup()

		group.AddSink(&sink1, receiveChan1)
		group.AddSink(&sink2, receiveChan2)

		msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "234", "App"), "origin")
		group.BroadcastMessage(msg)

		var nextChannelToReceive chan *events.Envelope
		var rmsg *events.Envelope
		select {
		case rmsg = <-receiveChan1:
			nextChannelToReceive = receiveChan2
		case rmsg = <-receiveChan2:
			nextChannelToReceive = receiveChan1
		}

		Expect(rmsg).To(Equal(msg))

		group.BroadcastMessage(msg)
		Expect(nextChannelToReceive).To(Receive(&msg))
	})

	It("does not send messages to unregistered sinks", func() {
		receiveChan1 := make(chan *events.Envelope, 10)
		receiveChan2 := make(chan *events.Envelope, 10)

		sink1 := fakeSink{appId: "firehose-a", sinkId: "sink-a"}
		sink2 := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

		group := firehose_group.NewFirehoseGroup()

		group.AddSink(&sink1, receiveChan1)
		group.AddSink(&sink2, receiveChan2)
		group.RemoveSink(&sink2)

		msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "234", "App"), "origin")

		group.BroadcastMessage(msg)
		Expect(receiveChan1).To(Receive(&msg))

		group.BroadcastMessage(msg)
		Expect(receiveChan1).To(Receive(&msg))
	})

	Describe("IsEmpty", func() {
		It("is true when the group is empty", func() {
			group := firehose_group.NewFirehoseGroup()
			Expect(group.IsEmpty()).To(BeTrue())
		})

		It("is false when the group is not empty", func() {
			group := firehose_group.NewFirehoseGroup()
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			Expect(group.IsEmpty()).To(BeFalse())
		})
	})

	Describe("RemoveSink", func() {
		It("makes the group empty and returns true when there is one sink to remove", func() {
			group := firehose_group.NewFirehoseGroup()
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			Expect(group.RemoveSink(&sink)).To(BeTrue())
			Expect(group.IsEmpty()).To(BeTrue())
		})

		It("returns false when the group does not contain the requested sink and does not remove any sinks from the group", func() {
			group := firehose_group.NewFirehoseGroup()
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			otherSink := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

			Expect(group.RemoveSink(&otherSink)).To(BeFalse())
			Expect(group.IsEmpty()).To(BeFalse())
		})
	})
})
