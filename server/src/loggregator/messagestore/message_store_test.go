package messagestore

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"testhelpers"
)

func TestDumpingMessagesForAnApp(t *testing.T) {
	messageString := "Some data"
	message := testhelpers.MarshalledLogMessage(t, messageString, "mySpace", "myApp", "myOrg")
	message2String := "Some more stuff"
	message2 := testhelpers.MarshalledLogMessage(t, message2String, "mySpace", "myApp", "myOrg")
	store := NewMessageStore(2)

	store.Add(message, "spaceId", "appId")
	store.Add(message2, "spaceId", "appId")

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor("spaceId", "appId"))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 2)
	testhelpers.AssertProtoBufferMessageEquals(t, messageString, messages[0])
	testhelpers.AssertProtoBufferMessageEquals(t, message2String, messages[1])
}

func TestFindingMessagesForAnotherApp(t *testing.T) {
	message := []byte("Hello world")
	store := NewMessageStore(1)

	store.Add(message, "spaceId", "appId")

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor("spaceId", "anotherAppId"))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 0)
}

func TestFindingMessagesForSpace(t *testing.T) {
	message := []byte("Hello world")
	store := NewMessageStore(1)

	store.Add(message, "spaceId", "appId")

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor("spaceId", "appId"))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, messages[0], message)
}
