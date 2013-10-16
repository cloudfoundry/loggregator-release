package sinkserver

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	otherMessageString := "Some more stuff"
	otherMessage := messagetesthelpers.MarshalledLogMessage(t, otherMessageString, "myApp")

	_, dontKeppAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
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

	dontKeppAliveChan <- true
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	_, stopKeepAlive1, _ := testhelpers.AddWSSink(t, client1ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	_, stopKeepAlive2, _ := testhelpers.AddWSSink(t, client2ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message from client 1.")
	case message := <-client1ReceivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message from client 2.")
	case message := <-client2ReceivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	stopKeepAlive1 <- true
	WaitForWebsocketRegistration()

	stopKeepAlive2 <- true
	WaitForWebsocketRegistration()
}

func TestThatItSendsLogsToProperAppSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherAppsMarshalledMessage := messagetesthelpers.MarshalledLogMessage(t, "Some other message", "otherApp")

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	_, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message from app sink.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	sink, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	dataReadChannel <- make([]byte, 10)

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Errorf("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}

	sink.Close()
	expectedMessageString := "My important message"
	mySpaceMarshalledMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	dataReadChannel <- mySpaceMarshalledMessage

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestDontDropSinkThatWorks(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")

	select {
	case <-time.After(200 * time.Millisecond):
	case <-droppedChannel:
		t.Errorf("Channel drop, but shouldn't have.")
	}

	expectedMessageString := "Some data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	dataReadChannel <- expectedMessage

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
	receivedChan := make(chan []byte, 2)
	_, _, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?")
	assert.Equal(t, true, <-droppedChannel)
}

func TestDropSinkWhenLogTargetisinvalid(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, TAIL_PATH+"invalidtarget", 4000)
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte)

	_, killKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	killKeepAliveChan <- true

	time.Sleep(60 * time.Millisecond) //wait a little bit to make sure the keep-alive has successfully been stopped

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	select {
	case msg1, ok := <-receivedChan:
		if ok {
			t.Errorf("We should not have received a message, but got: %v", msg1)
		}
	case <-time.After(10 * time.Millisecond):
		//no communication. That's good!
	}
}
