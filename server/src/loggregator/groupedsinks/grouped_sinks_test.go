package groupedsinks

import (
	"github.com/stretchr/testify/assert"
	"loggregator/sinks"
	"server_testhelpers"
	"testing"
)

var syslogWriter sinks.SyslogWriter

func init() {
	syslogWriter = sinks.NewSyslogWriter("tcp", "localhost:24631", "appId")
}

func TestRegisterAndFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := sinks.NewSyslogSink(appId, "url", server_testhelpers.Logger(), syslogWriter)
	result := groupedSinks.Register(appSink)

	appSinks := groupedSinks.For(appId)
	assert.True(t, result)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], appSink)
}

func TestRegisterReturnsFalseForEmptyAppId(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := ""
	appSink := sinks.NewSyslogSink(appId, "url", server_testhelpers.Logger(), syslogWriter)
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseForEmptyIdentifier(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "appId"
	appSink := sinks.NewSyslogSink(appId, "", server_testhelpers.Logger(), syslogWriter)
	result := groupedSinks.Register(appSink)

	assert.False(t, result)
}

func TestRegisterReturnsFalseWhenAttemptingToAddADuplicateSink(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appId := "789"
	appSink := sinks.NewSyslogSink(appId, "url", server_testhelpers.Logger(), syslogWriter)
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

	sink1 := sinks.NewSyslogSink(target, "url1", server_testhelpers.Logger(), syslogWriter)
	sink2 := sinks.NewSyslogSink(target, "url2", server_testhelpers.Logger(), syslogWriter)

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

	sink1 := sinks.NewDumpSink(target, 10, server_testhelpers.Logger())
	sink2 := sinks.NewSyslogSink(target, "url", server_testhelpers.Logger(), syslogWriter)
	sink3 := sinks.NewSyslogSink(otherTarget, "url", server_testhelpers.Logger(), syslogWriter)

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

	sink1 := sinks.NewSyslogSink(target, "other sink", server_testhelpers.Logger(), syslogWriter)
	sink2 := sinks.NewSyslogSink(target, "sink we are searching for", server_testhelpers.Logger(), syslogWriter)

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
	assert.Equal(t, sink2, sinkDrain)
}

func TestDumpForReturnsOnyDumps(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink(target, "url1", server_testhelpers.Logger(), syslogWriter)
	sink2 := sinks.NewSyslogSink(target, "url2", server_testhelpers.Logger(), syslogWriter)
	sink3 := sinks.NewDumpSink(target, 5, server_testhelpers.Logger())

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

	sink1 := sinks.NewDumpSink(target, 5, server_testhelpers.Logger())
	sink2 := sinks.NewDumpSink(otherTarget, 5, server_testhelpers.Logger())

	groupedSinks.Register(sink1)
	groupedSinks.Register(sink2)

	appSink := groupedSinks.DumpFor(target)
	assert.Equal(t, appSink, sink1)
}

func TestDumpForReturnsNilIfThereIsNone(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink(target, "url1", server_testhelpers.Logger(), syslogWriter)

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
