package groupedsinks_test

import (
	"doppler/groupedsinks"
	"doppler/sinks"
	"doppler/sinks/containermetric"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GroupedSink", func() {
	var groupedSinks *groupedsinks.GroupedSinks
	var inputChan chan *events.Envelope

	BeforeEach(func() {
		groupedSinks = groupedsinks.NewGroupedSinks(loggertesthelper.Logger())
		inputChan = make(chan *events.Envelope)
	})

	Describe("Broadcast", func() {
		Context("when all pre-existing firehose connections have been deleted", func() {
			It("sends message to all registered app sinks", func() {
				firehoseSink := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
				firehoseSinkChan := make(chan *events.Envelope, 2)
				groupedSinks.RegisterFirehoseSink(firehoseSinkChan, firehoseSink)

				groupedSinks.CloseAndDeleteFirehose(firehoseSink)

				appSink := syslog.NewSyslogSink("123", "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
				appSinkInputChan := make(chan *events.Envelope)
				groupedSinks.RegisterAppSink(appSinkInputChan, appSink)

				msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "123", "App"), "origin")
				go groupedSinks.Broadcast("123", msg)

				Expect(<-appSinkInputChan).To(Equal(msg))
			})
		})

		It("sends message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := syslog.NewSyslogSink("123", "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			otherInputChan := make(chan *events.Envelope)
			groupedSinks.RegisterAppSink(otherInputChan, appSink)

			appId = "789"
			appSink = syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, appSink)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			go groupedSinks.Broadcast(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("sends message to all registered firehose subscribers", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			inputChan1 := make(chan *events.Envelope, 2)
			groupedSinks.RegisterFirehoseSink(inputChan1, fakeSink1)

			fakeSink2 := &fakeSink{sinkId: "sink2", appId: "firehose-b"}
			inputChan2 := make(chan *events.Envelope, 2)
			groupedSinks.RegisterFirehoseSink(inputChan2, fakeSink2)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "app-id", "App"), "origin")
			go groupedSinks.Broadcast("app-id", msg)

			Eventually(inputChan2).Should(Receive(Equal(msg)))
			Eventually(inputChan1).Should(Receive(Equal(msg)))
		})

		It("distributes messages to all firehose sinks with the same subscription id", func() {
			fakeSink1A := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			inputChan1A := make(chan *events.Envelope, 100)
			groupedSinks.RegisterFirehoseSink(inputChan1A, fakeSink1A)

			fakeSink2A := &fakeSink{sinkId: "sink2", appId: "firehose-a"}
			inputChan2A := make(chan *events.Envelope, 100)
			groupedSinks.RegisterFirehoseSink(inputChan2A, fakeSink2A)

			fakeSinkB := &fakeSink{sinkId: "sink3", appId: "firehose-b"}
			inputChanB := make(chan *events.Envelope, 100)
			groupedSinks.RegisterFirehoseSink(inputChanB, fakeSinkB)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "app-id", "App"), "origin")
			for i := 0; i < 100; i++ {
				go groupedSinks.Broadcast("app-id", msg)
			}

			Eventually(func() int {
				return len(inputChan2A) + len(inputChan1A)
			}).Should(Equal(100))

			Consistently(func() int {
				return len(inputChan1A)
			}).Should(BeNumerically(">", 0))

			Consistently(func() int {
				return len(inputChan2A)
			}).Should(BeNumerically(">", 0))

			Eventually(func() int {
				return len(inputChanB)
			}).Should(Equal(100))
		})

		It("does not block when sending to an appId that has no sinks", func(done Done) {
			appId := "NonExistantApp"
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			groupedSinks.Broadcast(appId, msg)
			close(done)
		})
	})

	Describe("BroadcastError", func() {
		It("sends message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, make(chan int64))
			otherInputChan := make(chan *events.Envelope)
			groupedSinks.RegisterAppSink(otherInputChan, appSink)

			appId = "789"
			appSink = dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, appSink)
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "error message", appId, "App"), "origin")
			go groupedSinks.BroadcastError(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("sends message to all registered firehose subscribers", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			inputChan1 := make(chan *events.Envelope, 2)
			groupedSinks.RegisterFirehoseSink(inputChan1, fakeSink1)

			fakeSink2 := &fakeSink{sinkId: "sink2", appId: "firehose-b"}
			inputChan2 := make(chan *events.Envelope, 2)
			groupedSinks.RegisterFirehoseSink(inputChan2, fakeSink2)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "app-id", "App"), "origin")
			go groupedSinks.BroadcastError("app-id", msg)

			Eventually(inputChan2).Should(Receive(Equal(msg)))
			Eventually(inputChan1).Should(Receive(Equal(msg)))
		})

		It("does not send to sinks that don't want errors", func(done Done) {
			appId := "789"

			sink1 := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, make(chan int64))
			sink2 := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "error message", appId, "App"), "origin")
			go groupedSinks.BroadcastError(appId, msg)
			Expect(<-inputChan).To(Equal(msg))
			Expect(inputChan).To(HaveLen(0))
			close(done)
		})
	})

	Describe("Register", func() {
		It("returns false for empty app ids", func() {
			appId := ""
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			result := groupedSinks.RegisterAppSink(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("returns false for empty identifiers", func() {
			appId := "appId"
			appSink := syslog.NewSyslogSink(appId, "", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			result := groupedSinks.RegisterAppSink(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("returns false when registering a duplicate", func() {
			appId := "789"
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			groupedSinks.RegisterAppSink(inputChan, appSink)
			result := groupedSinks.RegisterAppSink(inputChan, appSink)
			Expect(result).To(BeFalse())
		})
	})

	Describe("RegisterFirehose", func() {
		It("returns false for empty subscription ids", func() {
			subscriptionId := ""
			firehoseSink := syslog.NewSyslogSink(subscriptionId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			result := groupedSinks.RegisterFirehoseSink(inputChan, firehoseSink)
			Expect(result).To(BeFalse())
		})

		It("returns true if a subscription id is present", func() {
			subscriptionId := "firehose-subscription-a"
			firehoseSink := syslog.NewSyslogSink(subscriptionId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			result := groupedSinks.RegisterFirehoseSink(inputChan, firehoseSink)
			Expect(result).To(BeTrue())
		})
	})

	Describe("CloseAndDelete", func() {
		It("only deletes a specific sink", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeTrue())
			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("handle deletes for non-existing appIds", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeFalse())

			Expect(groupedSinks.CountFor(target)).To(BeZero())
		})

		It("handle deletes for existing appIds but unregistered drain URLs", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)

			ok := groupedSinks.CloseAndDelete(sink2)
			Expect(ok).To(BeFalse())

			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("closes the inputChan", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			groupedSinks.RegisterAppSink(inputChan, sink)
			groupedSinks.CloseAndDelete(sink)
			Expect(inputChan).To(BeClosed())
		})

	})

	Describe("CloseAndDeleteFirehose", func() {
		It("only unregisters a specific sink", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			fakeSink2 := &fakeSink{sinkId: "sink2", appId: "firehose-a"}

			groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), fakeSink1)
			groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), fakeSink2)

			ok := groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(ok).To(BeTrue())
			Expect(groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), fakeSink1)).To(BeTrue())
			Expect(groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), fakeSink2)).To(BeFalse())
		})

		It("closes the sink's input channel", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			inputChan1 := make(chan *events.Envelope)

			groupedSinks.RegisterFirehoseSink(inputChan1, fakeSink1)

			groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(inputChan1).To(BeClosed())
		})

		It("returns false if the sink is not registered", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1", appId: "firehose-a"}
			ok := groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(ok).To(BeFalse())
		})
	})

	Describe("DeleteAll", func() {
		It("removes all the sinks", func() {
			sink1 := &fakeSink{sinkId: "sink1", appId: "app1"}
			sink2 := &fakeSink{sinkId: "sink2", appId: "app2"}
			sink3 := &fakeSink{sinkId: "sink3", appId: "firehose-a"}

			groupedSinks.RegisterAppSink(make(chan *events.Envelope), sink1)
			groupedSinks.RegisterAppSink(make(chan *events.Envelope), sink2)
			groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), sink3)

			groupedSinks.DeleteAll()

			Expect(groupedSinks.CountFor("123")).To(BeZero())
			Expect(groupedSinks.CountFor("465")).To(BeZero())
			Expect(groupedSinks.RegisterFirehoseSink(make(chan *events.Envelope), sink3)).To(BeTrue())
		})

		It("closes all the sinks input chans", func() {
			sink1 := &fakeSink{sinkId: "sink1", appId: "app1"}
			sink2 := &fakeSink{sinkId: "sink2", appId: "firehose-a"}

			groupedSinks.RegisterAppSink(inputChan, sink1)
			firehoseInputChan := make(chan *events.Envelope)
			groupedSinks.RegisterFirehoseSink(firehoseInputChan, sink2)

			groupedSinks.DeleteAll()

			Expect(inputChan).To(BeClosed())
			Expect(firehoseInputChan).To(BeClosed())
		})
	})

	Describe("DrainsFor", func() {
		It("does not return dump sinks", func() {
			target := "789"

			sink1 := dump.NewDumpSink(target, 10, loggertesthelper.Logger(), time.Second, make(chan int64))
			sink2 := syslog.NewSyslogSink(target, "url", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			sinkDrain := groupedSinks.DrainsFor(target)
			Expect(sinkDrain).To(HaveLen(1))
			Expect(sinkDrain[0]).To(Equal(sink2))
		})
	})

	Describe("DrainFor", func() {
		It("returns only sinks that match the appid and drain URL", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink2 := syslog.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
			Expect(sinkDrain).To(Equal(sink2))
		})

		It("returns nil if no drains are registered", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			groupedSinks.RegisterAppSink(inputChan, sink)

			Expect(groupedSinks.DrainFor(target, "url1")).To(BeNil())
		})

		It("return nils if no drains exist", func() {
			Expect(groupedSinks.DrainFor("empty", "empty")).To(BeNil())
		})
	})

	Describe("DumpFor", func() {
		It("returns only dumps", func() {
			appId := "789"

			sink1 := syslog.NewSyslogSink(appId, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink2 := syslog.NewSyslogSink(appId, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink3 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second, make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)
			groupedSinks.RegisterAppSink(inputChan, sink3)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink3))
		})

		It("returns only dumps that match the appId", func() {
			appId := "789"
			otherAppId := "790"

			sink1 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second, make(chan int64))
			sink2 := dump.NewDumpSink(otherAppId, 5, loggertesthelper.Logger(), time.Second, make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink1))
		})

		It("returns nil if no dumps are registered", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)

			Expect(groupedSinks.DumpFor(target)).To(BeNil())
		})

		It("returns nil if no sinks exist", func() {
			Expect(groupedSinks.DumpFor("empty")).To(BeNil())
		})
	})

	Describe("ContainerMetricsFor", func() {
		It("returns only container metric sinks", func() {
			appId := "456"

			sink1 := containermetric.NewContainerMetricSink(appId, 1*time.Second, time.Second, make(chan int64))
			sink2 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second, make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.ContainerMetricsFor(appId)).To(Equal(sink1))
		})

		It("returns only container metrics for appId", func() {
			appId1 := "123"
			appId2 := "456"

			sink1 := containermetric.NewContainerMetricSink(appId1, 1*time.Second, time.Second, make(chan int64))
			sink2 := containermetric.NewContainerMetricSink(appId2, 1*time.Second, time.Second, make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.ContainerMetricsFor(appId1)).To(Equal(sink1))
		})

		It("returns nil if no container metrics sinks are registered", func() {
			appId := "1234"
			sink2 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second, make(chan int64))
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.ContainerMetricsFor(appId)).To(BeNil())
		})

		It("returns nil if no sinks exist", func() {
			Expect(groupedSinks.ContainerMetricsFor("1234")).To(BeNil())
		})

	})

	Describe("WebsocketSinksFor", func() {
		It("returns only websocket sinks", func() {
			appId := "789"

			fakeWriter1 := fakeMessageWriter{RemoteAddress: "1"}
			fakeWriter2 := fakeMessageWriter{RemoteAddress: "2"}

			sink1 := syslog.NewSyslogSink(appId, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, dummyErrorHandler, "dropsonde-origin", make(chan int64))
			sink2 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter1, 100, "origin", make(chan int64))
			sink3 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter2, 100, "origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)
			groupedSinks.RegisterAppSink(inputChan, sink3)

			Expect(groupedSinks.WebsocketSinksFor(appId)).To(ConsistOf(*sink2, *sink3))
		})

		It("returns only sinks matching the app id", func() {
			appId := "789"
			otherAppId := "790"

			fakeWriter := fakeMessageWriter{RemoteAddress: "1"}

			sink1 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter, 100, "origin", make(chan int64))
			sink2 := websocket.NewWebsocketSink(otherAppId, loggertesthelper.Logger(), &fakeWriter, 100, "origin", make(chan int64))

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.WebsocketSinksFor(appId)).To(ConsistOf(*sink1))
		})

		It("returns an empty array if there are no matching sinks", func() {
			Expect(groupedSinks.WebsocketSinksFor("empty")).To(BeEmpty())
		})
	})
})

func dummyErrorHandler(_, _, _ string) {}

type DummySyslogWriter struct{}

func (d DummySyslogWriter) Connect() error { return nil }
func (d DummySyslogWriter) Write(p int, b []byte, source, sourceId string, timestamp int64) (int, error) {
	return 0, nil
}
func (d DummySyslogWriter) Close() error { return nil }

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
func (f *fakeSink) GetInstrumentationMetric() sinks.Metric {
	return sinks.Metric{Name: "numberOfMessagesLost", Value: 5}
}

func (f *fakeSink) UpdateDroppedMessageCount(messageCount int64) {}
