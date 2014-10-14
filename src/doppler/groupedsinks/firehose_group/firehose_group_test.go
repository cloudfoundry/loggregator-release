package firehose_group_test

import (
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"

	"doppler/groupedsinks/firehose_group"
	"doppler/groupedsinks/sink_wrapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeSink struct {
	sinkId string
	appId  string
}

func (f *fakeSink) StreamId() string {
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

var _ = Describe("FirehoseGroup", func() {
	It("sends message to all registered sinks", func() {
		receiveChan1 := make(chan *events.Envelope, 10)
		swrapper1 := &sink_wrapper.SinkWrapper{
			Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-a"},
			InputChan: receiveChan1,
		}

		receiveChan2 := make(chan *events.Envelope, 10)
		swrapper2 := &sink_wrapper.SinkWrapper{
			Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-b"},
			InputChan: receiveChan2,
		}

		group := firehose_group.NewFirehoseGroup()

		group.AddSink(swrapper1)
		group.AddSink(swrapper2)

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
		swrapper1 := &sink_wrapper.SinkWrapper{
			Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-a"},
			InputChan: receiveChan1,
		}

		receiveChan2 := make(chan *events.Envelope, 10)
		swrapper2 := &sink_wrapper.SinkWrapper{
			Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-b"},
			InputChan: receiveChan2,
		}

		group := firehose_group.NewFirehoseGroup()

		group.AddSink(swrapper1)
		group.AddSink(swrapper2)
		group.RemoveSink(swrapper2.Sink)

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

			swrapper1 := &sink_wrapper.SinkWrapper{
				Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-a"},
				InputChan: make(chan *events.Envelope, 10),
			}
			group.AddSink(swrapper1)

			Expect(group.IsEmpty()).To(BeFalse())
		})
	})

	Describe("RemoveSink", func() {
		It("makes the group empty and returns true when there is one sink to remove", func() {
			group := firehose_group.NewFirehoseGroup()
			swrapper := &sink_wrapper.SinkWrapper{
				Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-a"},
				InputChan: make(chan *events.Envelope, 10),
			}
			group.AddSink(swrapper)

			Expect(group.RemoveSink(swrapper.Sink)).To(BeTrue())
			Expect(group.IsEmpty()).To(BeTrue())
		})

		It("returns false when the group does not contain the requested sink and does not remove any sinks from the group", func() {
			group := firehose_group.NewFirehoseGroup()
			swrapper := &sink_wrapper.SinkWrapper{
				Sink:      &fakeSink{appId: "firehose-a", sinkId: "sink-a"},
				InputChan: make(chan *events.Envelope, 10),
			}
			group.AddSink(swrapper)

			otherSink := &fakeSink{appId: "firehose-a", sinkId: "sink-b"}

			Expect(group.RemoveSink(otherSink)).To(BeFalse())
			Expect(group.IsEmpty()).To(BeFalse())
		})
	})
})
