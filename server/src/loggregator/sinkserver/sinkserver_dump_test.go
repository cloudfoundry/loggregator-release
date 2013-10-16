package sinkserver

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func dumpAllMessages(receivedChan chan []byte) [][]byte {
	logMessages := [][]byte{}
	for message := range receivedChan {
		logMessages = append(logMessages, message)
	}
	return logMessages
}

func TestItDumpsAllMessagesForAnAppUser(t *testing.T) {
	expectedMessageString := "Some data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myOtherApp")

	dataReadChannel <- expectedMessage
	dataReadChannel <- expectedMessage

	receivedChan := make(chan []byte, 2)
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, DUMP_PATH+"?app=myOtherApp")

	logMessages := dumpAllMessages(receivedChan)

	assert.Equal(t, len(logMessages), 2)
	messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, logMessages[len(logMessages)-1])
	select {
	case <-droppedChannel:
		// we should have been dropped
	case <-time.After(10 * time.Millisecond):
		t.Error("we should have been dropped")
	}
	stopKeepAlive <- true
}

func TestDumpDropSinkWhenLogTargetisinvalid(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, DUMP_PATH+"invalidtarget", 4000)
}
