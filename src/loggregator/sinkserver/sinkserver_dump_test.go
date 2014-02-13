package sinkserver_test

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
	"time"
	"github.com/gorilla/websocket"
	"net/http"
)

func dumpAllMessages(receivedChan chan []byte) [][]byte {
	logMessages := [][]byte{}
	for message := range receivedChan {
		logMessages = append(logMessages, message)
	}
	return logMessages
}

func AssertConnectionFails(t *testing.T, port string, path string) {


}

func TestItDumpsAllMessagesForAnAppUser(t *testing.T) {
	expectedMessageString := "Some data"
	message := messagetesthelpers.NewMessage(t, expectedMessageString, "myOtherApp")

	dataReadChannel <- message
	dataReadChannel <- message

	receivedChan := make(chan []byte, 2)
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/dump/?app=myOtherApp")

	select {
	case <-droppedChannel:
		// we should have been dropped
	case <-time.After(10 * time.Millisecond):
		t.Error("we should have been dropped")
	}

	logMessages := dumpAllMessages(receivedChan)

	assert.Equal(t, len(logMessages), 2)
	messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, logMessages[len(logMessages)-1])

	stopKeepAlive <- true
}

func TestItDoesntHangWhenThereAreNoMessages(t *testing.T) {
	receivedChan := make(chan []byte, 1)
	testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/dump/?app=myOtherApp")

	doneChan := make(chan bool)
	go func() {
		dumpAllMessages(receivedChan)
		close(doneChan)
	}()
	select {
	case <-doneChan:
		break
	case <-time.After(10 * time.Millisecond):
		t.Error("Should have returned by now")
	}
}

func TestDumpDropSinkWhenLogTargetisinvalid(t *testing.T) {
	path := "/dump/?something=invalidtarget"
	_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+SERVER_PORT+path, http.Header{})
	assert.Error(t, err)
}
