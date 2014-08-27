package groupedsinks_test

import (
	"doppler/groupedsinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
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

type TestSink struct {
	appId, identifier string
}

func (c *TestSink) AppId() string { return c.appId }
func (c *TestSink) Run(msgChan <-chan *events.Envelope) {
	for _ = range msgChan {
	}
}

func (c *TestSink) Identifier() string        { return c.identifier }
func (c *TestSink) ShouldReceiveErrors() bool { return true }
func (c *TestSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
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

	Describe("DeleteAll", func() {
		It("removes all the sinks", func() {
			sink1 := &TestSink{"123", "url1"}
			sink2 := &TestSink{"465", "url2"}

			groupedSinks.Register(make(chan *events.Envelope), sink1)
			groupedSinks.Register(make(chan *events.Envelope), sink2)

			groupedSinks.DeleteAll()

			Expect(groupedSinks.CountFor("123")).To(BeZero())
			Expect(groupedSinks.CountFor("465")).To(BeZero())
		})

		It("closes all the sinks input chans", func() {
			sink := &TestSink{"123", "url1"}

			groupedSinks.Register(inputChan, sink)

			groupedSinks.DeleteAll()

			Eventually(inputChan).Should(BeClosed())
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
