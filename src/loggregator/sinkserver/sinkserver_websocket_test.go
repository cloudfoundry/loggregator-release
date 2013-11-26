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
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp01", SECRET)
	otherMessageString := "Some more stuff"
	otherLogEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, otherMessageString, "myApp01", SECRET)

	_, dontKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp01")
	WaitForWebsocketRegistration()

	dataReadChannel <- logEnvelope
	dataReadChannel <- otherLogEnvelope

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

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	_, stopKeepAlive1, _ := testhelpers.AddWSSink(t, client1ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	_, stopKeepAlive2, _ := testhelpers.AddWSSink(t, client2ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp")
	WaitForWebsocketRegistration()

	expectedMessageString := "Some Data"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp", SECRET)
	dataReadChannel <- logEnvelope

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

	otherLogMessage := messagetesthelpers.NewLogMessage("Some other message", "otherApp")
	otherLogEnvelope := messagetesthelpers.MarshalledLogEnvelope(t, otherLogMessage, SECRET)

	expectedMessageString := "My important message"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp02", SECRET)

	_, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp02")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherLogEnvelope
	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message from app sink.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestThatItDumpsLogsBeforeTailing(t *testing.T) {
	receivedChan := make(chan []byte)

	expectedMessageString := "My important message"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp06", SECRET)

	dataReadChannel <- logEnvelope

	_, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp06")
	WaitForWebsocketRegistration()

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message from app sink.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	expectedMessageString2 := "My Other important message"
	logEnvelope = messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString2, "myApp06", SECRET)

	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message from app sink.")
	case message := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString2, message)
	}

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	sink, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp03")
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
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp03", SECRET)
	dataReadChannel <- logEnvelope

	stopKeepAlive <- true
	WaitForWebsocketRegistration()
}

func TestDontDropSinkThatWorks(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp04")

	select {
	case <-time.After(200 * time.Millisecond):
	case <-droppedChannel:
		t.Errorf("Channel drop, but shouldn't have.")
	}

	expectedMessageString := "Some data"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp04", SECRET)
	dataReadChannel <- logEnvelope

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
	AssertConnectionFails(t, SERVER_PORT, TAIL_PATH+"?something=invalidtarget", 4000)
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte)

	_, killKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp05")
	WaitForWebsocketRegistration()

	killKeepAliveChan <- true

	time.Sleep(60 * time.Millisecond) //wait a little bit to make sure the keep-alive has successfully been stopped

	expectedMessageString := "My important message"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp05", SECRET)
	dataReadChannel <- logEnvelope

	select {
	case msg1, ok := <-receivedChan:
		if ok {
			t.Errorf("We should not have received a message, but got: %v", msg1)
		}
	case <-time.After(10 * time.Millisecond):
		//no communication. That's good!
	}
}
