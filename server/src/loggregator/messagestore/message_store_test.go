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

	messages, err := logmessage.ParseDumpedMessages(store.DumpFor(appId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, appMessage, messages[0].GetRawMessage())
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

	messages, err := logmessage.ParseDumpedMessages(store.DumpFor(appId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message, messages[0])

	messages, err = logmessage.ParseDumpedMessages(store.DumpFor(anotherAppId))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, anotherAppMessage, messages[0])
}

// This test exists because the ring buffer will dump messages
// that actually exist.
func TestOnlyDumpsMessagesThatHaveALength(t *testing.T) {
	store := NewMessageStore(2)

	target := "appId"
	message, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "Hello world", target))
	assert.NoError(t, err)
	store.Add(message, target)

	messages, err := logmessage.ParseDumpedMessages(store.DumpFor(target))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message, messages[0])
}
