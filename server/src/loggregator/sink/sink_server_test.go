package sink

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"strings"
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
	TestSinkServer = NewSinkServer(dataReadChannel, logger(), "localhost:8081", "/tail/", "http://localhost:9876", SuccessfulAuthorizer, 50*time.Millisecond)
	go TestSinkServer.Start()
	time.Sleep(1 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AddWSSink(t *testing.T, receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool) {
	dontKeepAliveChan := make(chan bool)
	ws, err := websocket.Dial("ws://localhost:"+port+path, "", "http://localhost")
	assert.NoError(t, err)
	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				t.Logf("Error while ws sink is receiving data: %v\n", err)
				break
			}

			receivedChan <- data
		}

	}()
	go func() {
		for {
			err := websocket.Message.Send(ws, []byte{42})
			if err != nil {
				t.Logf("Error while ws sink is receiving data: %v\n", err)
				break
			}
			select {
			case <-dontKeepAliveChan:
				return
			case <-time.After(44 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan
}

func AssertConnecitonFails(t *testing.T, port string, path string) {
	ws, err := websocket.Dial("ws://localhost:"+port+path, "", "http://localhost")
	assert.NoError(t, err)

	var data []byte
	err = websocket.Message.Receive(ws, &data)
	if err == nil {
		t.Errorf("Connection did not get closed.")
	}
}

func MarshalledLogMessage(t *testing.T, messageString string, spaceId string, appId string) []byte {
	currentTime := time.Now()

	messageType := logMessage.LogMessage_OUT
	sourceType := logMessage.LogMessage_DEA
	protoMessage := &logMessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		SpaceId:     proto.String(spaceId),
		MessageType: &messageType,
		SourceType:  &sourceType,
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}
	message, err := proto.Marshal(protoMessage)
	assert.NoError(t, err)

	return message
}

func AssertProtoBufferMessageEquals(t *testing.T, expectedMessage string, actual []byte) {
	receivedMessage := &logMessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}

func SuccessfulAuthorizer(a, b, c, d string, l *gosteno.Logger) bool {
	if b != "" {
		authString := strings.Split(b, " ")
		if len(authString) > 1 {
			return authString[1] == "correctAuthorizationToken"
		}
	}
	return false
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some data"
	expectedMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	otherMessageString := "Some more stuff"
	otherMessage := MarshalledLogMessage(t, otherMessageString, "mySpace", "myApp")

	AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
	dataReadChannel <- otherMessage

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message.")
	case message := <-receivedChan:
		AssertProtoBufferMessageEquals(t, expectedMessageString, message)
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get message.")
	case message := <-receivedChan:
		AssertProtoBufferMessageEquals(t, otherMessageString, message)
	}
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)
	space1ReceivedChan := make(chan []byte)
	space2ReceivedChan := make(chan []byte)

	AddWSSink(t, client1ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	AddWSSink(t, client2ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	AddWSSink(t, space1ReceivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	AddWSSink(t, space2ReceivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	WaitForWebsocketRegistration()

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	AssertProtoBufferMessageEquals(t, expectedMessageString, <-client1ReceivedChan)
	AssertProtoBufferMessageEquals(t, expectedMessageString, <-client2ReceivedChan)
	AssertProtoBufferMessageEquals(t, expectedMessageString, <-space1ReceivedChan)
	AssertProtoBufferMessageEquals(t, expectedMessageString, <-space2ReceivedChan)
}

func TestThatItSendsLogsToProperAppSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherAppsMarshalledMessage := MarshalledLogMessage(t, "Some other message", "mySpace", "otherApp")

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestThatItSendsLogsToProperSpaceSink(t *testing.T) {
	receivedChan := make(chan []byte)

	otherSpaceMarshalledMessage := MarshalledLogMessage(t, "Some other message", "otherSpace", "otherApp")

	expectedMessageString := "My important message"
	mySpaceMarshalledMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	WaitForWebsocketRegistration()

	dataReadChannel <- otherSpaceMarshalledMessage
	dataReadChannel <- mySpaceMarshalledMessage

	AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	sink, _ := AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
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
	mySpaceMarshalledMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	dataReadChannel <- mySpaceMarshalledMessage
}

func TestDropSinkWithoutAppAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/")
	TestThatItSends(t)
}

func TestDropSinkWithoutAuthorizationAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/spaces/mySpace/apps/myApp")
	TestThatItSends(t)
}

func TestDropSinkWhenAuthorizationFailsAndContinuesToWork(t *testing.T) {
	AssertConnecitonFails(t, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=incorrectAuthToken")
	TestThatItSends(t)
}

func TestKeepAlive(t *testing.T) {
	receivedChan := make(chan []byte)

	_, killKeepAliveChan := AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	WaitForWebsocketRegistration()

	killKeepAliveChan <- true

	time.Sleep(60 * time.Millisecond) //wait a little bit to make sure the keep-alive has successfully been stopped

	expectedMessageString := "My important message"
	myAppsMarshalledMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(10 * time.Millisecond) //wait a little bit to give a potential message time to arrive

	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}
