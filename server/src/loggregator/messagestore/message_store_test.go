package messagestore

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
)

func TestRegisterAndFor(t *testing.T) {
	store := NewMessageStore(2)

	appId := "myApp"
	appMessageString := "AppMessage"
	appMessage := testhelpers.MarshalledLogMessage(t, appMessageString, "myApp")
	message, err := logmessage.ParseMessage(appMessage)

	store.Add(message, appId)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(appId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)

	testhelpers.AssertProtoBufferMessageEquals(t, appMessageString, messages[0])
}

func TestAddingForAnotherApp(t *testing.T) {
	store := NewMessageStore(2)

	appId := "myApp"
	message, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "Message", appId))
	assert.NoError(t, err)
	store.Add(message, appId)

	anotherAppId := "anotherApp"
	anotherAppMessage, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "AnotherAppMessage", anotherAppId))
	assert.NoError(t, err)
	store.Add(anotherAppMessage, anotherAppId)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(appId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message.GetRawMessage(), messages[0])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(anotherAppId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, anotherAppMessage.GetRawMessage(), messages[0])
}

// This test exists because the ring buffer will dump messages
// that actually exist.
func TestOnlyDumpsMessagesThatHaveALength(t *testing.T) {
	store := NewMessageStore(2)

	target := "appId"
	message, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "Hello world", target))
	assert.NoError(t, err)
	store.Add(message, target)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(target))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, messages[0], message.GetRawMessage())
}
