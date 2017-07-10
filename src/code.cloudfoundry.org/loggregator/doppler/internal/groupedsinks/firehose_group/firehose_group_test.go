package firehose_group_test

import (
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"

	"code.cloudfoundry.org/loggregator/doppler/internal/groupedsinks/firehose_group"

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

var _ = Describe("FirehoseGroup", func() {
	It("sends message to all registered sinks", func() {
		receiveChan1 := make(chan *events.Envelope, 10)
		receiveChan2 := make(chan *events.Envelope, 10)

		sink1 := fakeSink{appId: "firehose-a", sinkId: "sink-a"}
		sink2 := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

		mc := testhelper.NewMetricClient()
		group := firehose_group.NewFirehoseGroup(
			&spyMetricBatcher{},
			mc.NewCounter("sinks.dropped",
				metricemitter.WithVersion(2, 0),
			),
		)

		group.AddSink(&sink1, receiveChan1)
		group.AddSink(&sink2, receiveChan2)

		msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "234", "App"), "origin")

		var (
			readFromChan1 bool
			readFromChan2 bool
		)
		f := func() bool {
			group.BroadcastMessage(msg)
			var rmsg *events.Envelope
			select {
			case rmsg = <-receiveChan1:
				readFromChan1 = true
			case rmsg = <-receiveChan2:
				readFromChan2 = true
			}
			Expect(rmsg).To(Equal(msg))
			return readFromChan1 && readFromChan2
		}
		Eventually(f).Should(BeTrue())
	})

	It("does not send messages to unregistered sinks", func() {
		receiveChan1 := make(chan *events.Envelope, 10)
		receiveChan2 := make(chan *events.Envelope, 10)

		sink1 := fakeSink{appId: "firehose-a", sinkId: "sink-a"}
		sink2 := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

		mc := testhelper.NewMetricClient()
		group := firehose_group.NewFirehoseGroup(
			&spyMetricBatcher{},
			mc.NewCounter("sinks.dropped",
				metricemitter.WithVersion(2, 0),
			),
		)

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
			mc := testhelper.NewMetricClient()
			group := firehose_group.NewFirehoseGroup(
				&spyMetricBatcher{},
				mc.NewCounter("sinks.dropped",
					metricemitter.WithVersion(2, 0),
				),
			)
			Expect(group.IsEmpty()).To(BeTrue())
		})

		It("is false when the group is not empty", func() {
			mc := testhelper.NewMetricClient()
			group := firehose_group.NewFirehoseGroup(
				&spyMetricBatcher{},
				mc.NewCounter("sinks.dropped",
					metricemitter.WithVersion(2, 0),
				),
			)
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			Expect(group.IsEmpty()).To(BeFalse())
		})
	})

	Describe("RemoveSink", func() {
		It("makes the group empty and returns true when there is one sink to remove", func() {
			mc := testhelper.NewMetricClient()
			group := firehose_group.NewFirehoseGroup(
				&spyMetricBatcher{},
				mc.NewCounter("sinks.dropped",
					metricemitter.WithVersion(2, 0),
				),
			)
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			Expect(group.RemoveSink(&sink)).To(BeTrue())
			Expect(group.IsEmpty()).To(BeTrue())
		})

		It("returns false when the group does not contain the requested sink and does not remove any sinks from the group", func() {
			mc := testhelper.NewMetricClient()
			group := firehose_group.NewFirehoseGroup(
				&spyMetricBatcher{},
				mc.NewCounter("sinks.dropped",
					metricemitter.WithVersion(2, 0),
				),
			)
			sink := fakeSink{appId: "firehose-a", sinkId: "sink-a"}

			group.AddSink(&sink, make(chan *events.Envelope, 10))

			otherSink := fakeSink{appId: "firehose-a", sinkId: "sink-b"}

			Expect(group.RemoveSink(&otherSink)).To(BeFalse())
			Expect(group.IsEmpty()).To(BeFalse())
		})
	})
})

type spyMetricBatcher struct{}

func (s *spyMetricBatcher) BatchIncrementCounter(name string) {}
