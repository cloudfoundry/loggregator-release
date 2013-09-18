package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"regexp"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

var TestMessageRouter *messageRouter
var TestHttpServer *httpServer
var dataReadChannel chan []byte

const (
	SERVER_PORT = "8081"
)

func init() {
	// This needs be unbuffered as the channel we get from the
	// agent listener is unbuffered?
	dataReadChannel = make(chan []byte, 10)
	TestMessageRouter = NewMessageRouter(1024, testhelpers.Logger())
	go TestMessageRouter.Start()
	TestHttpServer = NewHttpServer(TestMessageRouter, testhelpers.SuccessfulAuthorizer, 5*time.Millisecond, testhelpers.Logger())
	go TestHttpServer.Start(dataReadChannel, "localhost:"+SERVER_PORT)
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

// Metrics test

func TestMetrics(t *testing.T) {
	oldDumpSinksCounter := TestMessageRouter.Emit().Metrics[0].Value.(int)
	oldSyslogSinksCounter := TestMessageRouter.Emit().Metrics[1].Value.(int)
	oldWebsocketSinksCounter := TestMessageRouter.Emit().Metrics[2].Value.(int)

	client1ReceivedChan := make(chan []byte)
	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:32564")
	assert.NoError(t, err)
	go fakeSyslogDrain1.Serve()
	<-fakeSyslogDrain1.ReadyChan

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	marshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, "expectedMessageString", "myApp", "syslog://localhost:32564")
	dataReadChannel <- marshalledProtoBuffer

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-client1ReceivedChan:
	}

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	dataReadChannel <- marshalledProtoBuffer

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-client1ReceivedChan:
	}

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	receivedChan := make(chan []byte, 2)

	_, dontKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	WaitForWebsocketRegistration()

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter+1)

	dontKeepAliveChan <- true
	WaitForWebsocketRegistration()

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
	otherMessageString := "Some more stuff"
	otherMessage := messagetesthelpers.MarshalledLogMessage(t, otherMessageString, "myApp")

	_, dontKeppAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
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

	_, stopKeepAlive1, _ := testhelpers.AddWSSink(t, client1ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	_, stopKeepAlive2, _ := testhelpers.AddWSSink(t, client2ReceivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
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

	_, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
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

	sink, stopKeepAlive, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
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
	_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)

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
	_, _, droppedChannel := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	assert.Equal(t, true, <-droppedChannel)
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
	myAppsMarshalledMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")
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
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp543")

	dataReadChannel <- expectedMessage
	dataReadChannel <- expectedMessage

	req, err := http.NewRequest("GET", "http://localhost:"+SERVER_PORT+DUMP_PATH+"?app=myApp543", nil)
	assert.NoError(t, err)
	req.Header.Add("Authorization", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.Header.Get("Content-Type"), "application/octet-stream")
	assert.Equal(t, resp.StatusCode, 200)

	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	logMessages, err := logmessage.ParseDumpedLogMessages(body)
	assert.NoError(t, err)

	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, expectedMessageString, string(logMessages[len(logMessages)-1].GetMessage()))
}

func TestItReturns401WithIncorrectAuthToken(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:"+SERVER_PORT+DUMP_PATH+"?app=myApp543", nil)
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

// *** Start Syslog Sink tests
func TestThatItSendsAllMessageToKnownDrains(t *testing.T) {
	client1ReceivedChan := make(chan []byte)

	fakeSyslogDrain, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34566")
	defer fakeSyslogDrain.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34566")

	expectedSecondMessageString := "Some Data Without a drainurl"
	expectedSecondMarshalledProtoBuffer := messagetesthelpers.MarshalledLogMessage(t, expectedSecondMessageString, "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get the first message")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	dataReadChannel <- expectedSecondMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get the second message")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedSecondMessageString)
	}
}

func AssertMessageOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string, expectedMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		t.Errorf("%s", errorMessage)
	case message := <-receiveChan:
		assert.Contains(t, string(message), expectedMessage)
	}
}

func AssertMessageNotOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	case message := <-receiveChan:
		t.Errorf("%s %s", errorMessage, string(message))
	}
}

func TestThatItReestablishesConnectionToSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)

	fakeSyslogDrain, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString1 := "Some Data 1"
	expectedMarshalledProtoBuffer1 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString1, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer1

	errorString := "Did not get the first message. Server was up, it should have been there"
	AssertMessageOnChannel(t, 200, client1ReceivedChan, errorString, expectedMessageString1)
	fakeSyslogDrain.Stop()
	time.Sleep(50 * time.Millisecond)

	expectedMessageString2 := "Some Data 2"
	expectedMarshalledProtoBuffer2 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString2, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer2
	errorString = "Did get a second message! Shouldn't be since the server is down"
	AssertMessageNotOnChannel(t, 200, client1ReceivedChan, errorString)

	expectedMessageString3 := "Some Data 3"
	expectedMarshalledProtoBuffer3 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString3, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer3
	errorString = "Did get a third message! Shouldn't be since the server is down"
	AssertMessageNotOnChannel(t, 200, client1ReceivedChan, errorString)

	time.Sleep(2200 * time.Millisecond)

	client2ReceivedChan := make(chan []byte, 10)
	fakeSyslogDrain, err = NewFakeService(client2ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)

	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	time.Sleep(2200 * time.Millisecond)

	expectedMessageString4 := "Some Data 4"
	expectedMarshalledProtoBuffer4 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString4, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer4

	errorString = "Did not get the fourth message, but it should have been just fine since the server was up"
	AssertMessageOnChannel(t, 200, client2ReceivedChan, errorString, expectedMessageString4)

	fakeSyslogDrain.Stop()
}

func TestThatItSendsAllDataToAllDrainUrls(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34567")
	assert.NoError(t, err)
	go fakeSyslogDrain1.Serve()
	<-fakeSyslogDrain1.ReadyChan

	fakeSyslogDrain2, err := NewFakeService(client2ReceivedChan, "127.0.0.1:34568")
	assert.NoError(t, err)
	go fakeSyslogDrain2.Serve()
	<-fakeSyslogDrain2.ReadyChan

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34567", "syslog://localhost:34568")

	dataReadChannel <- expectedMarshalledProtoBuffer

	errString := "Did not get message from client 1."
	AssertMessageOnChannel(t, 200, client1ReceivedChan, errString, expectedMessageString)

	errString = "Did not get message from client 2."
	AssertMessageOnChannel(t, 200, client2ReceivedChan, errString, expectedMessageString)

	fakeSyslogDrain1.Stop()
	fakeSyslogDrain2.Stop()
}

func TestThatItSendsAllDataToOnlyAuthoritiveMessagesWithDrainUrls(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	defer fakeSyslogDrain1.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain1.Serve()
	<-fakeSyslogDrain1.ReadyChan

	fakeSyslogDrain2, err := NewFakeService(client2ReceivedChan, "127.0.0.1:34540")
	defer fakeSyslogDrain2.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain2.Serve()
	<-fakeSyslogDrain2.ReadyChan

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34569")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	expectedSecondMessageString := "loggregator myApp: loggregator myApp: Some More Data"
	expectedSecondMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedNonWardenLogMessage(t, expectedSecondMessageString, "myApp", "syslog://localhost:34540")

	dataReadChannel <- expectedSecondMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 2")
	case message := <-client1ReceivedChan:
		matched, _ := regexp.MatchString(`<6>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|-\d{2}:\d{2}) loggregator myApp: loggregator myApp: loggregator myApp: Some More Data`, string(message))
		assert.True(t, matched, string(message))
	case <-client2ReceivedChan:
		t.Error("Should not have gotten the new message in this drain")
	}
}

// *** End Syslog Sink tests
