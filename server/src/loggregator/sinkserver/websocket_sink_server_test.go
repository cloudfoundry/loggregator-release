package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"loggregator/messagestore"
	"net/http"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

var TestSinkServer *websocketSinkServer
var dataReadChannel chan []byte

const (
	SERVER_PORT = "8081"
)

func init() {
	// This needs be unbuffered as the channel we get from the
	// agent listener is unbuffered?
	dataReadChannel = make(chan []byte, 10)
	TestSinkServer = NewWebsocketSinkServer(dataReadChannel, messagestore.NewMessageStore(10), testhelpers.Logger(), "localhost:"+SERVER_PORT, testhelpers.SuccessfulAuthorizer, 50*time.Millisecond)
	go TestSinkServer.Start()
	time.Sleep(1 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnectionFails(t *testing.T, port string, path string, authToken string, expectedErrorCode uint16) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	if authToken != "" {
		config.Header.Add("Authorization", authToken)
	}

	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)
	data := make([]byte, 2)
	_, err = ws.Read(data)
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	expectedMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	otherMessageString := "Some more stuff"
	otherMessage := testhelpers.MarshalledLogMessage(t, otherMessageString, "myApp")

	testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	WaitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
	dataReadChannel <- otherMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message 1.")
	case message := <-receivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message 2.")
	case message := <-receivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, otherMessageString, message)
	}
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	testhelpers.AddWSSink(t, client1ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	testhelpers.AddWSSink(t, client2ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	WaitForWebsocketRegistration()

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message from client 1.")
	case message := <-client1ReceivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message from client 2.")
	case message := <-client2ReceivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}
}

func TestThatItSendsLogsToProperAppSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, "Some other message", "otherApp")

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	WaitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message from app sink.")
	case message := <-receivedChan:
		testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	sink, _, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
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
	mySpaceMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	dataReadChannel <- mySpaceMarshalledMessage
}

func TestDontDropSinkThatWorks(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	_, _, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)

	select {
	case <-time.After(200 * time.Millisecond):
	case <-droppedChannel:
		t.Errorf("Channel drop, but shouldn't have.")
	}

	TestThatItSends(t)
}

func TestQueryStringCombinationsThatDropSinkButContinueToWork(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	_, _, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	assert.Equal(t, true, <-droppedChannel)

	TestThatItSends(t)
}

var authTokenFailingCombinationTests = []struct {
	authToken string
}{
	{""},
	{testhelpers.INVALID_AUTHENTICATION_TOKEN},
}

func TestAuthTokenCombinationsThatDropSinkButContinueToWork(t *testing.T) {
	for _, test := range authTokenFailingCombinationTests {
		receivedChan := make(chan []byte, 2)
		_, _, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", test.authToken)
		assert.Equal(t, true, <-droppedChannel)

		TestThatItSends(t)
	}
}

func TestDropSinkWhenLogTargetisinvalidAndContinuesToWork(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, TAIL_PATH+"invalidtarget", "", 4000)
	TestThatItSends(t)
}

func TestDropSinkWithoutAuthorizationAndContinuesToWork(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, TAIL_PATH+"?app=myApp", "", 4001)
	TestThatItSends(t)
}

func TestDropSinkWhenAuthorizationFailsAndContinuesToWork(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.INVALID_AUTHENTICATION_TOKEN, 4002)
	TestThatItSends(t)
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte)

	_, killKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	WaitForWebsocketRegistration()

	killKeepAliveChan <- true

	time.Sleep(60 * time.Millisecond) //wait a little bit to make sure the keep-alive has successfully been stopped

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(10 * time.Millisecond) //wait a little bit to give a potential message time to arrive

	select {
	case msg1 := <-receivedChan:
		t.Errorf("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}

// *** Start dump tests

func TestItDumpsAllMessagesForAnAppUser(t *testing.T) {
	expectedMessageString := "Some data"
	expectedMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	dataReadChannel <- expectedMessage

	req, err := http.NewRequest("GET", "http://localhost:"+SERVER_PORT+DUMP_PATH+"?app=myApp", nil)
	assert.NoError(t, err)
	req.Header.Add("Authorization", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.Header.Get("Content-Type"), "application/octet-stream")
	assert.Equal(t, resp.StatusCode, 200)

	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	messages, err := testhelpers.ParseDumpedMessages(body)
	assert.NoError(t, err)

	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, messages[len(messages)-1])
}

func TestItReturns401WithIncorrectAuthToken(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:"+SERVER_PORT+DUMP_PATH+"?app=myApp", nil)
	assert.NoError(t, err)
	req.Header.Add("Authorization", testhelpers.INVALID_AUTHENTICATION_TOKEN)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 401)

	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "", string(body))
}

// *** End dump tests
