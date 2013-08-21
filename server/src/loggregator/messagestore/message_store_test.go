package messagestore

import (
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"github.com/stretchr/testify/assert"
	"testhelpers"
	"testing"
)

func TestRegisterAndFor(t *testing.T) {
	store := NewMessageStore(2)

	appTarget := &logtarget.LogTarget{AppId: "myApp"}
	appMessageString := "AppMessage"
	appMessage := testhelpers.MarshalledLogMessage(t, appMessageString, "myApp")

	store.Add(appMessage, appTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(appTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	testhelpers.AssertProtoBufferMessageEquals(t, appMessageString, messages[0])
}

func TestAddingForAnotherApp(t *testing.T) {
	store := NewMessageStore(2)

	appTarget := &logtarget.LogTarget{AppId: "myApp"}
	message := []byte("Message")
	store.Add(message, appTarget)

	anotherAppTarget := &logtarget.LogTarget{AppId: "anotherApp"}
	anotherAppMessage := []byte("AnotherAppMessage")
	store.Add(anotherAppMessage, anotherAppTarget)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(appTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, message, messages[0])

	messages, err = testhelpers.ParseDumpedMessages(store.DumpFor(anotherAppTarget))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, anotherAppMessage, messages[0])
}

// This test exists because the ring buffer will dump messages
// that actually exist.
func TestOnlyDumpsMessagesThatHaveALength(t *testing.T) {
	message := []byte("Hello world")
	store := NewMessageStore(2)

	target := &logtarget.LogTarget{AppId: "appId"}
	store.Add(message, target)

	messages, err := testhelpers.ParseDumpedMessages(store.DumpFor(target))
	assert.NoError(t, err)

	assert.Equal(t, len(messages), 1)
	assert.Equal(t, messages[0], message)
}
