package sinks_test

import (
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/router/internal/sinks"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GroupedSink", func() {
	var groupedSinks *sinks.GroupedSinks
	var inputChan chan *events.Envelope

	BeforeEach(func() {
		groupedSinks = sinks.NewGroupedSinks(
			testhelper.NewMetricClient(),
		)
		inputChan = make(chan *events.Envelope, 10)
	})

	Describe("Broadcast", func() {
		Context("when all pre-existing firehose connections have been deleted", func() {
			It("sends message to all registered app sinks", func() {
				appSink := sinks.NewDumpSink("123", 5, time.Hour, &spyHealthRegistrar{})
				appSinkInputChan := make(chan *events.Envelope, 10)
				groupedSinks.RegisterAppSink(appSinkInputChan, appSink)

				msg, _ := wrap(newLogMessage(events.LogMessage_OUT, "test message", "123", "App"), "origin")
				groupedSinks.Broadcast("123", msg)

				Eventually(appSinkInputChan).Should(Receive(Equal(msg)))
			})
		})

		It("sends message to all registered sinks that match the appId", func() {
			appId := "123"
			appSink := sinks.NewDumpSink(appId, 5, time.Hour, &spyHealthRegistrar{})

			otherInputChan := make(chan *events.Envelope)
			groupedSinks.RegisterAppSink(otherInputChan, appSink)

			appId = "789"
			appSink = sinks.NewDumpSink(appId, 5, time.Hour, &spyHealthRegistrar{})
			groupedSinks.RegisterAppSink(inputChan, appSink)

			msg, _ := wrap(newLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			groupedSinks.Broadcast(appId, msg)

			Eventually(inputChan).Should(Receive(Equal(msg)))
			Expect(otherInputChan).To(HaveLen(0))
		})

		It("does not block when sending to an appId that has no sinks", func(done Done) {
			appId := "NonExistantApp"
			msg, _ := wrap(newLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
			groupedSinks.Broadcast(appId, msg)
			close(done)
		})

		It("does not block when sending to slow sink", func() {
			appId := "syslog-a"
			fakeSink1A := &fakeSink{sinkId: "sink1", appId: appId}
			inputChan1A := make(chan *events.Envelope)
			groupedSinks.RegisterAppSink(inputChan1A, fakeSink1A)

			c := make(chan struct{})
			go func() {
				defer close(c)
				msg, _ := wrap(newLogMessage(events.LogMessage_OUT, "test message", appId, "App"), "origin")
				groupedSinks.Broadcast(appId, msg)
			}()

			Eventually(c).Should(BeClosed())
		})
	})

	Describe("Register", func() {
		It("returns false for empty app ids", func() {
			appId := ""
			appSink := sinks.NewDumpSink(appId, 5, time.Hour, &spyHealthRegistrar{})
			result := groupedSinks.RegisterAppSink(inputChan, appSink)
			Expect(result).To(BeFalse())
		})

		It("returns false when registering a duplicate", func() {
			appId := "789"
			appSink := sinks.NewDumpSink(appId, 5, time.Hour, &spyHealthRegistrar{})
			groupedSinks.RegisterAppSink(inputChan, appSink)
			result := groupedSinks.RegisterAppSink(inputChan, appSink)
			Expect(result).To(BeFalse())
		})
	})

	Describe("CloseAndDelete", func() {
		It("only deletes a specific sink", func() {
			target1 := "123"
			target2 := "789"

			sink1 := sinks.NewDumpSink(target1, 5, time.Hour, &spyHealthRegistrar{})
			sink2 := sinks.NewDumpSink(target2, 5, time.Hour, &spyHealthRegistrar{})

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeTrue())
		})

		It("handle deletes for non-existing appIds", func() {
			target := "789"
			sink1 := sinks.NewDumpSink(target, 5, time.Hour, &spyHealthRegistrar{})

			ok := groupedSinks.CloseAndDelete(sink1)
			Expect(ok).To(BeFalse())
		})

		It("closes the inputChan", func() {
			target := "789"
			sink := sinks.NewDumpSink(target, 5, time.Hour, &spyHealthRegistrar{})

			groupedSinks.RegisterAppSink(inputChan, sink)
			groupedSinks.CloseAndDelete(sink)
			Expect(inputChan).To(BeClosed())
		})

	})

	Describe("DeleteAll", func() {
		It("closes all the sinks input chans", func() {
			sink1 := &fakeSink{sinkId: "sink1", appId: "app1"}

			groupedSinks.RegisterAppSink(inputChan, sink1)

			groupedSinks.DeleteAll()

			Expect(inputChan).To(BeClosed())
		})
	})

	Describe("DumpFor", func() {
		It("returns only dumps", func() {
			appId := "789"
			health := newSpyHealthRegistrar()
			sink1 := sinks.NewDumpSink(appId, 5, time.Second, health)

			groupedSinks.RegisterAppSink(inputChan, sink1)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink1))
		})

		It("returns only dumps that match the appId", func() {
			appId := "789"
			otherAppId := "790"

			health := newSpyHealthRegistrar()
			sink1 := sinks.NewDumpSink(appId, 5, time.Second, health)
			sink2 := sinks.NewDumpSink(otherAppId, 5, time.Second, health)

			groupedSinks.RegisterAppSink(inputChan, sink1)
			groupedSinks.RegisterAppSink(inputChan, sink2)

			Expect(groupedSinks.DumpFor(appId)).To(Equal(sink1))
		})

		It("returns nil if no dumps are registered", func() {
			target := "789"

			Expect(groupedSinks.DumpFor(target)).To(BeNil())
		})

		It("returns nil if no sinks exist", func() {
			Expect(groupedSinks.DumpFor("empty")).To(BeNil())
		})
	})
})

func dummyErrorHandler(_, _ string) {}

type DummySyslogWriter struct{}

func (d DummySyslogWriter) Connect() error { return nil }
func (d DummySyslogWriter) Write(p int, b []byte, source, sourceId string, timestamp int64) (int, error) {
	return 0, nil
}
func (d DummySyslogWriter) Close() error { return nil }

type fakeMessageWriter struct {
	RemoteAddress string
}

func (fake *fakeMessageWriter) RemoteAddr() net.Addr {
	return fakeAddr{remoteAddress: fake.RemoteAddress}
}

func (fake *fakeMessageWriter) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (fake *fakeMessageWriter) SetWriteDeadline(t time.Time) error {
	return nil
}

type fakeAddr struct {
	remoteAddress string
}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}

func (fake fakeAddr) String() string {
	return fake.remoteAddress
}

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

type spyMetricBatcher struct{}

func (s *spyMetricBatcher) BatchIncrementCounter(name string) {}

type spyHealthRegistrar struct {
}

func (s *spyHealthRegistrar) Inc(name string) {
}

func (s *spyHealthRegistrar) Dec(name string) {
}
