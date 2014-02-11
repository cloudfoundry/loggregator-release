package groupedsinks

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"loggregator/sinks"
	"testing"
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

func TestRegisterAndFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	appSinks := groupedSinks.For(appId)
	assert.True(t, result)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], appSink)
}

func TestRegisterReturnsFalseForEmptyAppId(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := ""
	appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseForEmptyIdentifier(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "appId"
	appSink := syslog.NewSyslogSink(appId, "", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseWhenAttemptingToAddADuplicateSink(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := syslog.NewSyslogSink(appId, "url", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	groupedSinks.Register(appSink)
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestEmptyCollection(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	appId := "789"

	assert.Equal(t, len(groupedSinks.For(appId)), 0)
}

func TestDelete(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	groupedSinks.Delete(sink1)

	appSinks := groupedSinks.For(target)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], sink2)
}

func TestDrainsFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"
	otherTarget := "790"

	sink1 := dump.NewDumpSink(target, 10, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second)
	sink2 := syslog.NewSyslogSink(target, "url", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	sink3 := syslog.NewSyslogSink(otherTarget, "url", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)
	groupedSinks.Register(sink3)

	appSinks := groupedSinks.DrainsFor(target)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], sink2)
}

func TestDrainForReturnsOnly(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := syslog.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	sink2 := syslog.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
	assert.Equal(t, sink2, sinkDrain)
}

func TestDumpForReturnsOnyDumps(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	sink2 := syslog.NewSyslogSink(target, "url2", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))
	sink3 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second)

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)
	groupedSinks.Register(sink3)

	appSink := groupedSinks.DumpFor(target)
	assert.Equal(t, appSink, sink3)
}

func TestDumpForReturnsOnlyDumpsForTheGivenAppId(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"
	otherTarget := "790"

	sink1 := dump.NewDumpSink(target, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second)
	sink2 := dump.NewDumpSink(otherTarget, 5, loggertesthelper.Logger(), make(chan sinks.Sink, 1), time.Second)

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	appSink := groupedSinks.DumpFor(target)
	assert.Equal(t, appSink, sink1)
}

func TestDumpForReturnsNilIfThereIsNone(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := syslog.NewSyslogSink(target, "url1", loggertesthelper.Logger(), DummySyslogWriter{}, make(chan<- *logmessage.Message))

	groupedSinks.Register(sink1)

	appSink := groupedSinks.DumpFor(target)
	assert.Nil(t, appSink)
}

func TestDumpForReturnsNilIfThereIsNothing(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	appSink := groupedSinks.DumpFor(target)
	assert.Nil(t, appSink)
}
