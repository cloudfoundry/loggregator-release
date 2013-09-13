package messagestore

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddingAndDumping(t *testing.T) {
	appId := "myApp"
	expectedMessage := "AppMessage"

	store := NewMessageStore(1)
	store.Add(createMessage(t, appId, expectedMessage), appId)
	dump := store.DumpFor(appId)

	logMessages, err := logmessage.ParseDumpedLogMessages(dump)
	assert.NoError(t, err)
	assert.Equal(t, len(logMessages), 1)
	assert.Equal(t, expectedMessage, string(logMessages[0].GetMessage()))
}

func TestAddingForMultipleApps(t *testing.T) {
	store := NewMessageStore(2)

	appId := "myApp"
	message := createMessage(t, appId, "Message")

	anotherAppId := "anotherApp"
	anotherAppMessage := createMessage(t, anotherAppId, "AnotherAppMessage")

	store.Add(message, appId)
	store.Add(anotherAppMessage, anotherAppId)

	logMessages, err := logmessage.ParseDumpedLogMessages(store.DumpFor(appId))
	assert.NoError(t, err)
	assert.Equal(t, len(logMessages), 1)
	assert.Equal(t, "Message", string(logMessages[0].GetMessage()))

	logMessages, err = logmessage.ParseDumpedLogMessages(store.DumpFor(anotherAppId))
	assert.NoError(t, err)
	assert.Equal(t, len(logMessages), 1)
	assert.Equal(t, "AnotherAppMessage", string(logMessages[0].GetMessage()))
}

// This test exists because the ring buffer will dump messages
// that actually exist.
func TestOnlyDumpsMessagesThatHaveALength(t *testing.T) {
	store := NewMessageStore(200)

	appId := "appId"
	store.Add(createMessage(t, appId, "Hello world"), appId)

	logMessages, err := logmessage.ParseDumpedLogMessages(store.DumpFor(appId))
	assert.NoError(t, err)
	assert.Equal(t, len(logMessages), 1)
	assert.Equal(t, "Hello world", string(logMessages[0].GetMessage()))
}

func createMessage(t *testing.T, appId, messageString string) (message *logmessage.Message) {
	appMessage := messagetesthelpers.MarshalledLogMessage(t, messageString, appId)
	message, err := logmessage.ParseMessage(appMessage)
	assert.NoError(t, err)
	return
}
