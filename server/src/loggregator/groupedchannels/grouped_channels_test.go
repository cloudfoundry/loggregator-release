package groupedchannels

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"loggregator/logtarget"
)

func TestRegisterAndFor(t *testing.T) {
	groupedChannels := NewGroupedChannels()

	orgChannel := make(chan []byte)
	targetWithOnlyOrg := &logtarget.LogTarget{OrgId: "123"}
	groupedChannels.Register(orgChannel, targetWithOnlyOrg)

	spaceChannel := make(chan []byte)
	targetWithOrgAndSpace := &logtarget.LogTarget{OrgId: "123", SpaceId: "456"}
	groupedChannels.Register(spaceChannel, targetWithOrgAndSpace)

	appChannel := make(chan []byte)
	targetWithOrgSpaceAndApp := &logtarget.LogTarget{OrgId: "123", SpaceId: "456", AppId: "789"}
	groupedChannels.Register(appChannel, targetWithOrgSpaceAndApp)

	orgSpaceAppChannels := groupedChannels.For(targetWithOrgSpaceAndApp)
	assert.Equal(t, len(orgSpaceAppChannels), 3)
	assert.Equal(t, orgSpaceAppChannels[0], orgChannel)
	assert.Equal(t, orgSpaceAppChannels[1], spaceChannel)
	assert.Equal(t, orgSpaceAppChannels[2], appChannel)

	orgSpaceChannels := groupedChannels.For(targetWithOrgAndSpace)
	assert.Equal(t, len(orgSpaceChannels), 2)
	assert.Equal(t, orgSpaceChannels[0], orgChannel)
	assert.Equal(t, orgSpaceChannels[1], spaceChannel)

	orgChannels := groupedChannels.For(targetWithOnlyOrg)
	assert.Equal(t, len(orgChannels), 1)
	assert.Equal(t, orgChannels[0], orgChannel)
}

func TestEmptyCollection(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	targetWithOnlyOrg := &logtarget.LogTarget{OrgId: "123"}
	targetWithOrgAndSpace := &logtarget.LogTarget{OrgId: "123", SpaceId: "456"}
	targetWithOrgSpaceAndApp := &logtarget.LogTarget{OrgId: "123", SpaceId: "456", AppId: "789"}

	assert.Equal(t, len(groupedChannels.For(targetWithOnlyOrg)), 0)
	assert.Equal(t, len(groupedChannels.For(targetWithOrgAndSpace)), 0)
	assert.Equal(t, len(groupedChannels.For(targetWithOrgSpaceAndApp)), 0)
}

func TestDeleteForOrg(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	target := &logtarget.LogTarget{OrgId: "123"}

	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Register(channel1, target)
	groupedChannels.Register(channel2, target)

	groupedChannels.Delete(channel1)

	orgChannels := groupedChannels.For(target)
	assert.Equal(t, len(orgChannels), 1)
	assert.Equal(t, orgChannels[0], channel2)
}

func TestDeleteForOrgSpace(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	target := &logtarget.LogTarget{OrgId: "123", SpaceId: "456"}

	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Register(channel1, target)
	groupedChannels.Register(channel2, target)

	groupedChannels.Delete(channel1)

	orgChannels := groupedChannels.For(target)
	assert.Equal(t, len(orgChannels), 1)
	assert.Equal(t, orgChannels[0], channel2)
}

func TestDeleteForOrgSpaceApp(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	target := &logtarget.LogTarget{OrgId: "123", SpaceId: "456", AppId: "789"}

	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Register(channel1, target)
	groupedChannels.Register(channel2, target)

	groupedChannels.Delete(channel1)

	orgChannels := groupedChannels.For(target)
	assert.Equal(t, len(orgChannels), 1)
	assert.Equal(t, orgChannels[0], channel2)
}

func TestTotalNumberOfChannels(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	targetWithOnlyOrg := &logtarget.LogTarget{OrgId: "123"}
	groupedChannels.Register(channel1, targetWithOnlyOrg)

	channel2 := make(chan []byte)
	targetWithOrgAndSpace := &logtarget.LogTarget{OrgId: "123", SpaceId: "456"}
	groupedChannels.Register(channel2, targetWithOrgAndSpace)

	channel3 := make(chan []byte)
	targetWithOrgSpaceAndApp := &logtarget.LogTarget{OrgId: "123", SpaceId: "456", AppId: "789"}
	groupedChannels.Register(channel3, targetWithOrgSpaceAndApp)


	assert.Equal(t, groupedChannels.NumberOfChannels(), 3)

	groupedChannels.Delete(channel1)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 2)

	groupedChannels.Delete(channel2)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 1)

	groupedChannels.Delete(channel3)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 0)
}
