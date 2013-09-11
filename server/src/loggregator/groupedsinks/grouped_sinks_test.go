package groupedsinks

import (
	"github.com/stretchr/testify/assert"
	"loggregator/sinks"
	"server_testhelpers"
	"testing"
)

func TestRegisterAndFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()

	appSink := *new(sinks.Sink)
	appId := "789"
	groupedSinks.Register(appSink, appId)

	appSinks := groupedSinks.For(appId)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], appSink)
}

func TestEmptyCollection(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	appId := "789"

	assert.Equal(t, len(groupedSinks.For(appId)), 0)
}

func TestDelete(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink("1", "url", server_testhelpers.Logger())
	sink2 := sinks.NewSyslogSink("2", "url", server_testhelpers.Logger())

	groupedSinks.Register(sink1, target)
	groupedSinks.Register(sink2, target)

	groupedSinks.Delete(sink1)

	appSinks := groupedSinks.For(target)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], sink2)
}

func TestDrainsFor(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := *new(sinks.Sink)
	sink2 := sinks.NewSyslogSink("1", "url", server_testhelpers.Logger())

	groupedSinks.Register(sink1, target)
	groupedSinks.Register(sink2, target)

	appSinks := groupedSinks.DrainsFor(target)
	assert.Equal(t, len(appSinks), 1)
	assert.Equal(t, appSinks[0], sink2)
}

func TestDrainForReturnsOnly(t *testing.T) {
	groupedSinks := NewGroupedSinks()
	target := "789"

	sink1 := sinks.NewSyslogSink("1", "other sink", server_testhelpers.Logger())
	sink2 := sinks.NewSyslogSink("2", "sink we are searching for", server_testhelpers.Logger())

	groupedSinks.Register(sink1, target)
	groupedSinks.Register(sink2, target)

	sinkDrain := groupedSinks.DrainFor(target, "sink we are searching for")
	assert.Equal(t, sink2, sinkDrain)
}
