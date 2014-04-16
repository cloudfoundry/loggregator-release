package websocketserver_test

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func AssertWsConnectionFails(t *testing.T, port string, path string) {
	_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
	assert.Error(t, err)

}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	message := messagetesthelpers.NewMessage(t, expectedMessageString, "myApp01")
	otherMessageString := "Some more stuff"
	otherMessage := messagetesthelpers.NewMessage(t, otherMessageString, "myApp01")

	_, dontKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/tail/?app=myApp01")
	WaitForWebsocketRegistration()

	dataReadChannel <- message
	dataReadChannel <- otherMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message 1.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message 2.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, otherMessageString, message)
	}

	dontKeepAliveChan <- true
}

func TestThatItDoesNotDumpLogsBeforeTailing(t *testing.T) {
	receivedChan := make(chan []byte)

	expectedMessageString := "My important message"
	message := messagetesthelpers.NewMessage(t, expectedMessageString, "myApp06")

	dataReadChannel <- message

	_, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/tail/?app=myApp06")
	WaitForWebsocketRegistration()

	select {
	case <-time.After(1 * time.Second):
		break
	case _, ok := <-receivedChan:
		if ok {
			t.Errorf("Recieved unexpected message from app sink")
		}
	}

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestDontDropSinkThatWorks(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/tail/?app=myApp04")

	select {
	case <-time.After(200 * time.Millisecond):
	case <-droppedChannel:
		t.Errorf("Channel drop, but shouldn't have.")
	}

	expectedMessageString := "Some data"
	message := messagetesthelpers.NewMessage(t, expectedMessageString, "myApp04")
	dataReadChannel <- message

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestQueryStringCombinationsThatDropSinkButContinueToWork(t *testing.T) {
	AssertWsConnectionFails(t, SERVER_PORT, "/tail/?")
}

func TestDropSinkWhenLogTargetisinvalid(t *testing.T) {
	AssertWsConnectionFails(t, SERVER_PORT, "/tail/?something=invalidtarget")
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte, 10)

	_, killKeepAliveChan, connectionDroppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, "/tail/?app=myApp05")
	WaitForWebsocketRegistration()

	go func() {
		for {
			expectedMessageString := "My important message"
			message := messagetesthelpers.NewMessage(t, expectedMessageString, "myApp05")
			dataReadChannel <- message
			time.Sleep(2 * time.Millisecond)
		}
	}()

	time.Sleep(10 * time.Millisecond) //wait a little bit to make sure some messages are sent

	killKeepAliveChan <- true

	go func() {
		for {
			select {

			case _, ok := <-receivedChan:
				if !ok {
					// channel closed good!
					break
				}
			case <-time.After(10 * time.Millisecond):
				//no communication. That's good!
				break
			}
		}
	}()

	select {
	case fu := <-connectionDroppedChannel:
		assert.True(t, fu, "We should have been dropped since we stopped the keepalive")
	case <-time.After(1 * time.Second):
		t.Fatal("Should have read from connnectionDropppedChannel by now")
	}

}
