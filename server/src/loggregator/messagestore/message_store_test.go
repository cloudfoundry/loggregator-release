package messagestore

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"testhelpers"
	"loggregator/logtarget"
)

func TestRegisterAndFor(t *testing.T) {
	store := NewMessageStore(2)

	orgSpaceAppTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace", AppId: "myApp"}
	orgSpaceAppMessageString := "OrgSpaceAppMessage"
	orgSpaceAppMessage := testhelpers.MarshalledLogMessage(t, orgSpaceAppMessageString, "mySpace", "myApp", "myOrg")

	orgSpaceTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace"}
	orgSpaceMessageString := "OrgSpaceMessage"
	orgSpaceMessage := testhelpers.MarshalledLogMessage(t, orgSpaceMessageString, "mySpace", "", "myOrg")


	orgTarget := &logtarget.LogTarget{OrgId: "myOrg"}
	orgMessageString := "OrgMessage"
	orgMessage := testhelpers.MarshalledLogMessage(t, orgMessageString, "", "", "myOrg")

	store.Add(orgMessage, orgTarget)
	store.Add(orgSpaceMessage, orgSpaceTarget)
	store.Add(orgSpaceAppMessage, orgSpaceAppTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(orgTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 3)
	testhelpers.AssertProtoBufferMessageEquals(t, orgMessageString, messages[0])
	testhelpers.AssertProtoBufferMessageEquals(t, orgSpaceMessageString, messages[1])
	testhelpers.AssertProtoBufferMessageEquals(t, orgSpaceAppMessageString, messages[2])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(orgSpaceTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 2)
	testhelpers.AssertProtoBufferMessageEquals(t, orgSpaceMessageString, messages[0])
	testhelpers.AssertProtoBufferMessageEquals(t, orgSpaceAppMessageString, messages[1])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(orgSpaceAppTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	testhelpers.AssertProtoBufferMessageEquals(t, orgSpaceAppMessageString, messages[0])
}

func TestAddingForAnotherOrg(t *testing.T) {
	store := NewMessageStore(2)

	orgTarget := &logtarget.LogTarget{OrgId: "myOrg"}
	orgMessage := []byte("MyOrgMessage")
	store.Add(orgMessage, orgTarget)

	anotherOrgTarget := &logtarget.LogTarget{OrgId: "anotherOrg"}
	anotherOrgMessage := []byte("AnotherOrgMessage")
	store.Add(anotherOrgMessage, anotherOrgTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(orgTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, orgMessage, messages[0])
}

func TestAddingForAnotherSpace(t *testing.T) {
	store := NewMessageStore(2)

	orgTarget := &logtarget.LogTarget{OrgId: "myOrg"}

	spaceTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace"}
	message := []byte("Message")
	store.Add(message, spaceTarget)

	anotherSpaceTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "anotherSpace"}
	anotherSpaceMessage := []byte("AnotherSpaceMessage")
	store.Add(anotherSpaceMessage, anotherSpaceTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(spaceTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message, messages[0])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(orgTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 2)
	assert.Equal(t, message, messages[0])
	assert.Equal(t, anotherSpaceMessage, messages[1])
}

func TestAddingForAnotherApp(t *testing.T) {
	store := NewMessageStore(2)

	orgTarget := &logtarget.LogTarget{OrgId: "myOrg"}
	spaceTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace"}

	appTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace", AppId: "myApp"}
	message := []byte("Message")
	store.Add(message, appTarget)

	anotherAppTarget := &logtarget.LogTarget{OrgId: "myOrg", SpaceId: "mySpace", AppId: "anotherApp"}
	anotherAppMessage := []byte("AnotherAppMessage")
	store.Add(anotherAppMessage, anotherAppTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(appTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message, messages[0])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(spaceTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 2)
	assert.Equal(t, message, messages[0])
	assert.Equal(t, anotherAppMessage, messages[1])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(orgTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 2)
	assert.Equal(t, message, messages[0])
	assert.Equal(t, anotherAppMessage, messages[1])
}

// This test exists because the ring buffer will dump messages
// that actually exist.
func TestOnlyDumpsMessagesThatHaveALength(t *testing.T) {
	message := []byte("Hello world")
	store := NewMessageStore(2)

	target := &logtarget.LogTarget{OrgId: "orgId", SpaceId:"spaceId", AppId:"appId" }
	store.Add(message, target)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(target))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, messages[0], message)
}
