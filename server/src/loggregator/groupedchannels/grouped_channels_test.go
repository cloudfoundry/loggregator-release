package groupedchannels

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRegisterAndFor(t *testing.T) {
	groupedChannels := NewGroupedChannels()

	appChannel := make(chan *logmessage.Message)
	appId := "789"
	groupedChannels.Register(appChannel, appId)

	appChannels := groupedChannels.For(appId)
	assert.Equal(t, len(appChannels), 1)
	assert.Equal(t, appChannels[0], appChannel)
}

func TestEmptyCollection(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	appId := "789"

	assert.Equal(t, len(groupedChannels.For(appId)), 0)
}

func TestDeleteForOrgSpaceApp(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	target := "789"

	channel1 := make(chan *logmessage.Message)
	channel2 := make(chan *logmessage.Message)

	groupedChannels.Register(channel1, target)
	groupedChannels.Register(channel2, target)

	groupedChannels.Delete(channel1)

	appChannels := groupedChannels.For(target)
	assert.Equal(t, len(appChannels), 1)
	assert.Equal(t, appChannels[0], channel2)
}

func TestTotalNumberOfChannels(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan *logmessage.Message)
	appId1 := "1"
	groupedChannels.Register(channel1, appId1)

	channel2 := make(chan *logmessage.Message)
	appId2 := "2"
	groupedChannels.Register(channel2, appId2)

	channel3 := make(chan *logmessage.Message)
	appId3 := "3"
	groupedChannels.Register(channel3, appId3)

	assert.Equal(t, groupedChannels.NumberOfChannels(), 3)

	groupedChannels.Delete(channel1)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 2)

	groupedChannels.Delete(channel2)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 1)

	groupedChannels.Delete(channel3)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 0)
}
