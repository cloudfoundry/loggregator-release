package groupedchannels

import (
	"github.com/stretchr/testify/assert"
	"loggregator/logtarget"
	"testing"
)

func TestRegisterAndFor(t *testing.T) {
	groupedChannels := NewGroupedChannels()

	appChannel := make(chan []byte)
	targetWithApp := &logtarget.LogTarget{AppId: "789"}
	groupedChannels.Register(appChannel, targetWithApp)

	appChannels := groupedChannels.For(targetWithApp)
	assert.Equal(t, len(appChannels), 1)
	assert.Equal(t, appChannels[0], appChannel)
}

func TestEmptyCollection(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	targetWithApp := &logtarget.LogTarget{AppId: "789"}

	assert.Equal(t, len(groupedChannels.For(targetWithApp)), 0)
}

func TestDeleteForOrgSpaceApp(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	target := &logtarget.LogTarget{AppId: "789"}

	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Register(channel1, target)
	groupedChannels.Register(channel2, target)

	groupedChannels.Delete(channel1)

	appChannels := groupedChannels.For(target)
	assert.Equal(t, len(appChannels), 1)
	assert.Equal(t, appChannels[0], channel2)
}

func TestTotalNumberOfChannels(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	targetWithApp1 := &logtarget.LogTarget{AppId: "1"}
	groupedChannels.Register(channel1, targetWithApp1)

	channel2 := make(chan []byte)
	targetWithApp2 := &logtarget.LogTarget{AppId: "2"}
	groupedChannels.Register(channel2, targetWithApp2)

	channel3 := make(chan []byte)
	targetWithApp3 := &logtarget.LogTarget{AppId: "3"}
	groupedChannels.Register(channel3, targetWithApp3)

	assert.Equal(t, groupedChannels.NumberOfChannels(), 3)

	groupedChannels.Delete(channel1)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 2)

	groupedChannels.Delete(channel2)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 1)

	groupedChannels.Delete(channel3)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 0)
}
