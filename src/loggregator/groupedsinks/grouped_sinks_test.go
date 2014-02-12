package groupedsinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"loggregator/groupedsinks"
	"loggregator/sinks"
	"time"
	"loggregator/sinks/syslog"
	"loggregator/sinks/dump"
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

var _ = Describe("GroupedSink", func() {
	var fakeTimeProvider *faketimeprovider.FakeTimeProvider
	var groupedSinks *groupedsinks.GroupedSinks
	var inputChan, errorChan chan *logmessage.Message


	BeforeEach(func(){
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

	Describe("BroadCastError", func(){
			It("should send message to all registered sinks that match the appId", func(done Done) {
				appId := "123"
				appSink := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)
				otherInputChan := make(chan *logmessage.Message)
				groupedSinks.Register(otherInputChan, appSink)

				appId = "789"
				appSink = dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)

				groupedSinks.Register(inputChan, appSink)
				msg := NewMessage("error message", appId)
				go groupedSinks.BroadCastError(appId, msg)

				Expect(<-inputChan).To(Equal(msg))
				Expect(otherInputChan).To(HaveLen(0))
				close(done)
			})

			It("should not send to sinks that don't want errors", func(done Done){
				appId := "789"

				sink1 := dump.NewDumpSink(appId, 10, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)
				sink2 := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

				groupedSinks.Register(inputChan,sink1)
				groupedSinks.Register(inputChan,sink2)
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

		It("should return false when registring a duplicate", func(){
			appId := "789"
			appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			groupedSinks.Register(inputChan, appSink)
			result := groupedSinks.Register(inputChan, appSink)
			Expect(result).To(BeFalse())
		})
	})

	Describe("Delete", func() {
		It("should delete all sinks for the appId", func(){
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan,sink1)
			groupedSinks.Register(inputChan,sink2)

			groupedSinks.Delete(sink1)
			Expect(groupedSinks.CountFor(target)).To(Equal(1))
		})

		It("should close the inputChan", func() {
			target := "789"

			sink := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			groupedSinks.Register(inputChan,sink)
			groupedSinks.Delete(sink)
			Expect(inputChan).To(BeClosed())
		})
	})

	Describe("DrainsFor", func(){
		It("should not return dump sinks", func(){
			target := "789"

			sink1 := dump.NewDumpSink(target, 10, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)
			sink2 := syslog.NewSyslogSink(target, "url", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan,sink1)
			groupedSinks.Register(inputChan,sink2)

			sinkDrain := groupedSinks.DrainsFor(target)
			Expect(sinkDrain).To(HaveLen(1))
			Expect(sinkDrain[0]).To(Equal(sink2))
		})

		It("should return only sinks that match the appid", func() {
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)

			groupedSinks.Register(inputChan,sink1)
			groupedSinks.Register(inputChan,sink2)

			sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
			Expect(sinkDrain).To(Equal(sink2))
		})
	})

	Describe("DumpFor", func(){
		It("should return only dumps", func(){
			target := "789"

			sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, errorChan)
			sink3 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)

			groupedSinks.Register(inputChan,sink1)
			groupedSinks.Register(inputChan,sink2)
			groupedSinks.Register(inputChan,sink3)

			Expect(groupedSinks.DumpFor(target)).To(Equal(sink3))
		})

		It("should return only dumps that match the appId", func(){
			target := "789"
			otherTarget := "790"

			sink1 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)
			sink2 := dump.NewDumpSink(otherTarget, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second, fakeTimeProvider)

			groupedSinks.Register(inputChan,sink1)
			groupedSinks.Register(inputChan,sink2)

			Expect(groupedSinks.DumpFor(target)).To(Equal(sink1))
		})

		It("should return nil if no dumps are registered", func(){
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
