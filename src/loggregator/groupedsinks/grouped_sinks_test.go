package groupedsinks

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"loggregator/sinks"
	"testing"
)

var syslogWriter sinks.SyslogWriter

func init() {
	syslogWriter = sinks.NewSyslogWriter("syslog", "localhost:24631", "appId", false)
}

func TestRegisterAndFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := sinks.NewSyslogSink(appId, "url", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	appSinks := groupedSinks.For(appId)
	assert.True(t, result)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], appSink)
}

func TestRegisterReturnsFalseForEmptyAppId(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := ""
	appSink := sinks.NewSyslogSink(appId, "url", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseForEmptyIdentifier(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "appId"
	appSink := sinks.NewSyslogSink(appId, "", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseWhenAttemptingToAddADuplicateSink(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := sinks.NewSyslogSink(appId, "url", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
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

	sink1 := sinks.NewSyslogSink(target, "url1", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	sink2 := sinks.NewSyslogSink(target, "url2", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))

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

	sink1 := sinks.NewDumpSink(target, 10, loggertesthelper.Logger())
	sink2 := sinks.NewSyslogSink(target, "url", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	sink3 := sinks.NewSyslogSink(otherTarget, "url", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))

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

	sink1 := sinks.NewSyslogSink(target, "other sink", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	sink2 := sinks.NewSyslogSink(target, "sink we are searching for", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
	assert.Equal(t, sink2, sinkDrain)
}

func TestDumpForReturnsOnyDumps(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink(target, "url1", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	sink2 := sinks.NewSyslogSink(target, "url2", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))
	sink3 := sinks.NewDumpSink(target, 5, loggertesthelper.Logger())

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

	sink1 := sinks.NewDumpSink(target, 5, loggertesthelper.Logger())
	sink2 := sinks.NewDumpSink(otherTarget, 5, loggertesthelper.Logger())

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	appSink := groupedSinks.DumpFor(target)
	assert.Equal(t, appSink, sink1)
}

func TestDumpForReturnsNilIfThereIsNone(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink(target, "url1", loggertesthelper.Logger(), syslogWriter, make(chan<- *logmessage.Message))

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
