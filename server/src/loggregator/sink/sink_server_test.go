package sink

import (
	"code.google.com/p/go.net/websocket"
	"github.com/stretchr/testify/assert"
	"testhelpers"
	"testing"
	"time"
)

var TestSinkServer *sinkServer
var dataReadChannel chan []byte

func init() {
	// This needs be unbuffered as the channel we get from the
	// agent listener is unbuffered?
	//	dataReadChannel = make(chan []byte, 10)
	dataReadChannel = make(chan []byte, 10)
	TestSinkServer = NewSinkServer(dataReadChannel, testhelpers.Logger(), "localhost:8081", "/tail/", "http://localhost:9876", testhelpers.SuccessfulAuthorizer, 50*time.Millisecond)
	go TestSinkServer.Start()
	time.Sleep(1 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnecitonFails(t *testing.T, port string, path string, authToken string) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	if authToken != "" {
		config.Header.Add("Authorization", authToken)
	}

	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)
	var data []byte
	err = websocket.Message.Receive(ws, &data)
	if err == nil {
		t.Errorf("Connection did not get closed.")
	}
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	expectedMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	otherMessageString := "Some more stuff"
	otherMessage := testhelpers.MarshalledLogMessage(t, otherMessageString, "mySpace", "myApp")

	testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
	dataReadChannel <- otherMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message.")
	case message := <-receivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message.")
	case message := <-receivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, otherMessageString, message)
	}
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)
	space1ReceivedChan := make(chan []byte)
	space2ReceivedChan := make(chan []byte)

	testhelpers.AddWSSink(t, client1ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	testhelpers.AddWSSink(t, client2ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	testhelpers.AddWSSink(t, space1ReceivedChan, "8081", "/tail/spaces/mySpace", "bearer correctAuthorizationToken")
	testhelpers.AddWSSink(t, space2ReceivedChan, "8081", "/tail/spaces/mySpace", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-client1ReceivedChan)
	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-client2ReceivedChan)
	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-space1ReceivedChan)
	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-space2ReceivedChan)
}

func TestThatItSendsLogsToProperAppSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, "Some other message", "mySpace", "otherApp")

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestThatItSendsLogsToProperSpaceSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherSpaceMarshalledMessage := testhelpers.MarshalledLogMessage(t, "Some other message", "otherSpace", "otherApp")

	expectedMessageString := "My important message"
	mySpaceMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherSpaceMarshalledMessage
	dataReadChannel <- mySpaceMarshalledMessage

	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	sink, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- make([]byte, 10)

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}

	sink.Close()
	expectedMessageString := "My important message"
	mySpaceMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	dataReadChannel <- mySpaceMarshalledMessage
}

func TestDropSinkWithoutAppAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/", "")
	TestThatItSends(t)
}

func TestDropSinkWithoutAuthorizationAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/spaces/mySpace/apps/myApp", "")
	TestThatItSends(t)
}

func TestDropSinkWhenAuthorizationFailsAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/spaces/mySpace/apps/myApp", "incorrectAuthorizationToken")
	TestThatItSends(t)
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte)

	_, killKeepAliveChan := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp", "bearer correctAuthorizationToken")
	WaitForWebsocketRegistration()

	killKeepAliveChan <- true

	time.Sleep(60 * time.Millisecond) //wait a little bit to make sure the keep-alive has successfully been stopped

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(10 * time.Millisecond) //wait a little bit to give a potential message time to arrive

	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}
