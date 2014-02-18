package groupedsinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/groupedsinks"
	"loggregator/sinks/dump"
	"loggregator/sinks/syslog"
	"time"
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
func (c *TestSink) Run(msgChan <-chan *logmessage.Message) {
	for _ = range msgChan {
	}
}

func (c *TestSink) Identifier() string        { return c.identifier }
func (c *TestSink) ShouldReceiveErrors() bool { return true }
func (c *TestSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

var _ = Describe("GroupedSink", func() {
	var fakeTimeProvider *faketimeprovider.FakeTimeProvider
	var groupedSinks *groupedsinks.GroupedSinks
	var inputChan, errorChan chan *logmessage.Message

	BeforeEach(func() {
		fakeTimeProvider = faketimeprovider.New(time.Now())
		groupedSinks = groupedsinks.NewGroupedSinks()
		inputChan = make(chan *logmessage.Message)
		errorChan = make(chan *logmessage.Message)

	})

	Describe("BroadCast", func() {
		It("should send message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := syslog.NewSyslogSink("123", "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			otherInputChan := make(chan *logmessage.Message)
			groupedSinks.Register(otherInputChan, appSink)

			appId = "789"
			appSink = syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, appSink)

			msg := NewMessage("test message", appId)
			go groupedSinks.BroadCast(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("should not block when sending to an appId that has no sinks", func(done Done) {
			appId := "NonExistantApp"
			msg := NewMessage("test message", appId)
			groupedSinks.BroadCast(appId, msg)
			close(done)
		})
	})

	Describe("BroadCastError", func() {
		It("should send message to all registered sinks that match the appId", func(done Done) {
			appId := "123"
			appSink := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
			otherInputChan := make(chan *logmessage.Message)
			groupedSinks.Register(otherInputChan, appSink)

			appId = "789"
			appSink = dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

			groupedSinks.Register(inputChan, appSink)
			msg := NewMessage("error message", appId)
			go groupedSinks.BroadCastError(appId, msg)

			Expect(<-inputChan).To(Equal(msg))
			Expect(otherInputChan).To(HaveLen(0))
			close(done)
		})

		It("should not send to sinks that don't want errors", func(done Done) {
			appId := "789"

			sink1 := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
			sink2 := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)
			msg := NewMessage("error message", appId)
			go groupedSinks.BroadCastError(appId, msg)
			Expect(<-inputChan).To(Equal(msg))
			Expect(inputChan).To(HaveLen(0))
			close(done)
		})
	})

	Describe("Register", func() {
		It("should return false for empty app ids", func() {
			appId := ""
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("should return false for emtpy identifiers", func() {
			appId := "appId"
			appSink := syslog.NewSyslogSink(appId, "", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("should return false when registring a duplicate", func() {
			appId := "789"
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			groupedSinks.Register(inputChan, appSink)
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should only delete a specific sink", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			ok := groupedSinks.Delete(sink1)
			Expect(ok).To(BeTrue())
			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("should handle delete for non-existing appIds", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			ok := groupedSinks.Delete(sink1)
			Expect(ok).To(BeFalse())

			Expect(groupedSinks.CountFor(target)).To(BeZero())
		})

		It("should handle delete for existing appIds but unregistered drain URLs", func() {
				target := "789"

				sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
				sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

				groupedSinks.Register(inputChan, sink1)

				ok := groupedSinks.Delete(sink2)
				Expect(ok).To(BeFalse())

				Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})


		It("should close the inputChan", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			groupedSinks.Register(inputChan, sink)
			groupedSinks.Delete(sink)
			Expect(inputChan).To(BeClosed())
		})

	})

	Describe("DeleteAll", func() {
		It("should remove all the sinks", func() {
			sink1 := &TestSink{"123", "url1"}
			sink2 := &TestSink{"465", "url2"}

			groupedSinks.Register(make(chan *logmessage.Message), sink1)
			groupedSinks.Register(make(chan *logmessage.Message), sink2)

			groupedSinks.DeleteAll()

			Expect(groupedSinks.CountFor("123")).To(BeZero())
			Expect(groupedSinks.CountFor("465")).To(BeZero())
		})

		It("should close all the sinks input chans", func() {
			sink := &TestSink{"123", "url1"}

			groupedSinks.Register(inputChan, sink)

			groupedSinks.DeleteAll()

			Eventually(inputChan).Should(BeClosed())
		})
	})

	Describe("DrainsFor", func() {
		It("should not return dump sinks", func() {
			target := "789"

			sink1 := dump.NewDumpSink(target, 10, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
			sink2 := syslog.NewSyslogSink(target, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			sinkDrain := groupedSinks.DrainsFor(target)
			Expect(sinkDrain).To(HaveLen(1))
			Expect(sinkDrain[0]).To(Equal(sink2))
		})
	})

	Describe("DrainFor", func() {
		It("should return only sinks that match the appid and drain URL", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
			Expect(sinkDrain).To(Equal(sink2))
		})

		It("should return nil if no drains are registered", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			groupedSinks.Register(inputChan, sink)

			Expect(groupedSinks.DrainFor(target, "url1")).To(BeNil())
		})

		It("should return nil if no drains exist", func() {
			Expect(groupedSinks.DrainFor("empty", "empty")).To(BeNil())
		})
	})

	Describe("DumpFor", func() {
		It("should return only dumps", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink3 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)
			groupedSinks.Register(inputChan, sink3)

			Expect(groupedSinks.DumpFor(target)).To(Equal(sink3))
		})

		It("should return only dumps that match the appId", func() {
			target := "789"
			otherTarget := "790"

			sink1 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
			sink2 := dump.NewDumpSink(otherTarget, 5, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

			groupedSinks.Register(inputChan, sink1)
			groupedSinks.Register(inputChan, sink2)

			Expect(groupedSinks.DumpFor(target)).To(Equal(sink1))
		})

		It("should return nil if no dumps are registered", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan, sink1)

			Expect(groupedSinks.DumpFor(target)).To(BeNil())
		})

		It("should return nil if no sinks exist", func() {
			Expect(groupedSinks.DumpFor("empty")).To(BeNil())
		})
	})
})
