package groupedsinks_test

import (
	"doppler/groupedsinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"doppler/sinks/websocket"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type DummySyslogWriter struct{}

func (d DummySyslogWriter) Connect() error { return nil }
func (d DummySyslogWriter) WriteStdout(b []byte, source, sourceId string, timestamp int64) (int, error) {
	return 0, nil
}
func (d DummySyslogWriter) WriteStderr(b []byte, source, sourceId string, timestamp int64) (int, error) {
	return 0, nil
}
func (d DummySyslogWriter) Close() error      { return nil }
func (d DummySyslogWriter) IsConnected() bool { return false }
func (d DummySyslogWriter) SetConnected(bool) {}

type fakeSink struct {
	sinkId string
	appId  string
}

func (f *fakeSink) AppId() string {
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

var _ = Describe("GroupedSink", func() {
	var groupedSinks *groupedsinks.GroupedSinks
	var inputChan, errorChan chan *events.Envelope

	BeforeEach(func() {
		groupedSinks = groupedsinks.NewGroupedSinks()
		inputChan = make(chan *events.Envelope)
		errorChan = make(chan *events.Envelope)

	})

	Describe("BroadCast", func() {
		It("sends message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := syslog.NewSyslogSink("123", "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			otherInputChan := make(chan *events.Envelope)
			groupedSinks.Register(otherInputChan, appSink)

			appId = "789"
			appSink = syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, appSink)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			go groupedSinks.BroadCast(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("sends message to all registered firehose sinks", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1"}
			inputChan1 := make(chan *events.Envelope)
			groupedSinks.RegisterFirehose(inputChan1, fakeSink1)

			fakeSink2 := &fakeSink{sinkId: "sink2"}
			inputChan2 := make(chan *events.Envelope)
			groupedSinks.RegisterFirehose(inputChan2, fakeSink2)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "app-id", "App"), "origin")
			go groupedSinks.BroadCast("app-id", msg)

			Eventually(inputChan1).Should(Receive(Equal(msg)))
			Eventually(inputChan2).Should(Receive(Equal(msg)))
		})

		It("does not block when sending to an appId that has no sinks", func(done Done) {
			appId := "NonExistantApp"
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			groupedSinks.BroadCast(appId, msg)
			close(done)
		})
	})

	Describe("BroadCastError", func() {
		It("sends message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second)
			otherInputChan := make(chan *events.Envelope)
			groupedSinks.Register(otherInputChan, appSink)

			appId = "789"
			appSink = dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second)

			groupedSinks.Register(inputChan, appSink)
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "error message", appId, "App"), "origin")
			go groupedSinks.BroadCastError(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("sends message to all registered firehose sinks", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1"}
			inputChan1 := make(chan *events.Envelope)
			groupedSinks.RegisterFirehose(inputChan1, fakeSink1)

			fakeSink2 := &fakeSink{sinkId: "sink2"}
			inputChan2 := make(chan *events.Envelope)
			groupedSinks.RegisterFirehose(inputChan2, fakeSink2)

			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "app-id", "App"), "origin")
			go groupedSinks.BroadCastError("app-id", msg)

			Eventually(inputChan1).Should(Receive(Equal(msg)))
			Eventually(inputChan2).Should(Receive(Equal(msg)))
		})

		It("does not send to sinks that don't want errors", func(done Done) {
			appId := "789"

			sink1 := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second)
			sink2 := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)
			msg, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "error message", appId, "App"), "origin")
			go groupedSinks.BroadCastError(appId, msg)
			Expect(<-inputChan).To(Equal(msg))
			Expect(inputChan).To(HaveLen(0))
			close(done)
		})
	})

	Describe("Register", func() {
		It("returns false for empty app ids", func() {
			appId := ""
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("returns false for emtpy identifiers", func() {
			appId := "appId"
			appSink := syslog.NewSyslogSink(appId, "", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("returns false when registering a duplicate", func() {
			appId := "789"
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			groupedSinks.Register(inputChan, appSink)
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})
	})

	Describe("RegisterFirehose", func() {
		It("returns false when registering a duplicate", func() {
			fakeSink := &fakeSink{sinkId: "sink"}
			ok := groupedSinks.RegisterFirehose(inputChan, fakeSink)
			Expect(ok).To(BeTrue())
			ok = groupedSinks.RegisterFirehose(inputChan, fakeSink)
			Expect(ok).To(BeFalse())
		})
	})

	Describe("CloseAndDelete", func() {
		It("only deletes a specific sink", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeTrue())
			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("handle deletes for non-existing appIds", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeFalse())

			Expect(groupedSinks.CountFor(target)).To(BeZero())
		})

		It("handle deletes for existing appIds but unregistered drain URLs", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)

			ok := groupedSinks.CloseAndDelete(sink2)
			Expect(ok).To(BeFalse())

			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("closes the inputChan", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			groupedSinks.Register(inputChan, sink)
			groupedSinks.CloseAndDelete(sink)
			Expect(inputChan).To(BeClosed())
		})

	})

	Describe("CloseAndDeleteFirehose", func() {
		It("only unregisters a specific sink", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1"}
			fakeSink2 := &fakeSink{sinkId: "sink2"}

			groupedSinks.RegisterFirehose(make(chan *events.Envelope), fakeSink1)
			groupedSinks.RegisterFirehose(make(chan *events.Envelope), fakeSink2)

			ok := groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(ok).To(BeTrue())
			Expect(groupedSinks.RegisterFirehose(make(chan *events.Envelope), fakeSink1)).To(BeTrue())
			Expect(groupedSinks.RegisterFirehose(make(chan *events.Envelope), fakeSink2)).To(BeFalse())
		})

		It("closes the sink's input channel", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1"}
			inputChan1 := make(chan *events.Envelope)

			groupedSinks.RegisterFirehose(inputChan1, fakeSink1)

			groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(inputChan1).To(BeClosed())
		})

		It("returns false if the sink is not registered", func() {
			fakeSink1 := &fakeSink{sinkId: "sink1"}
			ok := groupedSinks.CloseAndDeleteFirehose(fakeSink1)
			Expect(ok).To(BeFalse())
		})
	})

	Describe("DeleteAll", func() {
		It("removes all the sinks", func() {
			sink1 := &fakeSink{sinkId: "sink1", appId: "app1"}
			sink2 := &fakeSink{sinkId: "sink2", appId: "app2"}
			sink3 := &fakeSink{sinkId: "sink3"}

			groupedSinks.Register(make(chan *events.Envelope), sink1)
			groupedSinks.Register(make(chan *events.Envelope), sink2)
			groupedSinks.RegisterFirehose(make(chan *events.Envelope), sink3)

			groupedSinks.DeleteAll()

			Expect(groupedSinks.CountFor("123")).To(BeZero())
			Expect(groupedSinks.CountFor("465")).To(BeZero())
			Expect(groupedSinks.RegisterFirehose(make(chan *events.Envelope), sink3)).To(BeTrue())
		})

		It("closes all the sinks input chans", func() {
			sink1 := &fakeSink{sinkId: "sink1", appId: "app1"}
			sink2 := &fakeSink{sinkId: "sink2"}

			groupedSinks.Register(inputChan, sink1)
			firehoseInputChan := make(chan *events.Envelope)
			groupedSinks.RegisterFirehose(firehoseInputChan, sink2)

			groupedSinks.DeleteAll()

			Expect(inputChan).To(BeClosed())
			Expect(firehoseInputChan).To(BeClosed())
		})
	})

	Describe("DrainsFor", func() {
		It("does not return dump sinks", func() {
			target := "789"

			sink1 := dump.NewDumpSink(target, 10, loggertesthelper.Logger(), time.Second)
			sink2 := syslog.NewSyslogSink(target, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			sinkDrain := groupedSinks.DrainsFor(target)
			Expect(sinkDrain).To(HaveLen(1))
			Expect(sinkDrain[0]).To(Equal(sink2))
		})
	})

	Describe("DrainFor", func() {
		It("returns only sinks that match the appid and drain URL", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink2 := syslog.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
			Expect(sinkDrain).To(Equal(sink2))
		})

		It("returns nil if no drains are registered", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			groupedSinks.Register(inputChan, sink)

			Expect(groupedSinks.DrainFor(target, "url1")).To(BeNil())
		})

		It("return nils if no drains exist", func() {
			Expect(groupedSinks.DrainFor("empty", "empty")).To(BeNil())
		})
	})

	Describe("DumpFor", func() {
		It("returns only dumps", func() {
			appId := "789"

			sink1 := syslog.NewSyslogSink(appId, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink2 := syslog.NewSyslogSink(appId, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink3 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)
			groupedSinks.Register(inputChan, sink3)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink3))
		})

		It("returns only dumps that match the appId", func() {
			appId := "789"
			otherAppId := "790"

			sink1 := dump.NewDumpSink(appId, 5, loggertesthelper.Logger(), time.Second)
			sink2 := dump.NewDumpSink(otherAppId, 5, loggertesthelper.Logger(), time.Second)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink1))
		})

		It("returns nil if no dumps are registered", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")

			groupedSinks.Register(inputChan, sink1)

			Expect(groupedSinks.DumpFor(target)).To(BeNil())
		})

		It("returns nil if no sinks exist", func() {
			Expect(groupedSinks.DumpFor("empty")).To(BeNil())
		})
	})

	Describe("WebsocketSinksFor", func() {
		It("returns only websocket sinks", func() {
			appId := "789"

			fakeWriter1 := fakeMessageWriter{RemoteAddress: "1"}
			fakeWriter2 := fakeMessageWriter{RemoteAddress: "2"}

			sink1 := syslog.NewSyslogSink(appId, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan, "dropsonde-origin")
			sink2 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter1, 100, "origin")
			sink3 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter2, 100, "origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)
			groupedSinks.Register(inputChan, sink3)

			Expect(groupedSinks.WebsocketSinksFor(appId)).To(ConsistOf(*sink2, *sink3))
		})

		It("returns only sinks matching the app id", func() {
			appId := "789"
			otherAppId := "790"

			fakeWriter := fakeMessageWriter{RemoteAddress: "1"}

			sink1 := websocket.NewWebsocketSink(appId, loggertesthelper.Logger(), &fakeWriter, 100, "origin")
			sink2 := websocket.NewWebsocketSink(otherAppId, loggertesthelper.Logger(), &fakeWriter, 100, "origin")

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			Expect(groupedSinks.WebsocketSinksFor(appId)).To(ConsistOf(*sink1))
		})

		It("returns an empty array if there are no matching sinks", func() {
			Expect(groupedSinks.WebsocketSinksFor("empty")).To(BeEmpty())
		})
	})
})
