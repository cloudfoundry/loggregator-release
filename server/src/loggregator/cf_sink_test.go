package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"testing"
	"time"
)

func waitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func addWSSink(t *testing.T, receivedChan chan []byte, port string, path string) (ws *websocket.Conn) {
	ws, err := websocket.Dial("ws://localhost:"+port+path, "string", "http://localhost")
	assert.NoError(t, err)

	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				break
			}
			receivedChan <- data
		}
	}()
	return ws
}

func marshalledLogMessage(t *testing.T, messageString string, spaceId string, appId string) []byte {
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

func assertProtoBufferMessageEquals(t *testing.T, expectedMessage string, actual []byte) {
	receivedMessage := &logMessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}

func successfulAuthorizer(a, b, c, d string, l *gosteno.Logger) bool {
	return true
}

func failingAuthorizer(a, b, c, d string, l *gosteno.Logger) bool {
	return false
}

var sinkServer *cfSinkServer
var dataReadChannel chan []byte

func init() {
	dataReadChannel = make(chan []byte, 10)
	sinkServer = NewCfSinkServer(dataReadChannel, logger(), "localhost:8081", "/tail/", "http://localhost:9876", successfulAuthorizer)
	go sinkServer.Start()
	time.Sleep(1 * time.Millisecond)
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some Data"
	expectedMessage := marshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")
	otherMessageString := "More stuff"
	otherMessage := marshalledLogMessage(t, otherMessageString, "mySpace", "myApp")

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
	dataReadChannel <- otherMessage

	assertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
	assertProtoBufferMessageEquals(t, otherMessageString, <-receivedChan)
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)
	space1ReceivedChan := make(chan []byte)
	space2ReceivedChan := make(chan []byte)

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := marshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	wsClient1 := addWSSink(t, client1ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer wsClient1.Close()

	wsClient2 := addWSSink(t, client2ReceivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer wsClient2.Close()
	waitForWebsocketRegistration()

	wsSpaceClient1 := addWSSink(t, space1ReceivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	defer wsSpaceClient1.Close()
	waitForWebsocketRegistration()

	wsSpaceClient2 := addWSSink(t, space2ReceivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	defer wsSpaceClient2.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- expectedMarshalledProtoBuffer

	assertProtoBufferMessageEquals(t, expectedMessageString, <-client1ReceivedChan)
	assertProtoBufferMessageEquals(t, expectedMessageString, <-client2ReceivedChan)
	assertProtoBufferMessageEquals(t, expectedMessageString, <-space1ReceivedChan)
	assertProtoBufferMessageEquals(t, expectedMessageString, <-space2ReceivedChan)
}

func TestThatItSendsLogsForOneApplication(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	otherAppsMarshalledMessage := marshalledLogMessage(t, "Some other message", "mySpace", "otherApp")
	expectedMessageString := "My important message"
	myAppsMarshalledMessage := marshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	assertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestThatItSendsLogsForOneSpace(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	otherAppsMarshalledMessage := marshalledLogMessage(t, "Some other message", "mySpace", "otherApp")
	expectedMessageString := "My important message"
	myAppsMarshalledMessage := marshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	fmt.Println("Adding sink")

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage

	fmt.Println("One down one to go")

	dataReadChannel <- myAppsMarshalledMessage

	fmt.Println("Sent messages")

	assertProtoBufferMessageEquals(t, "Some other message", <-receivedChan)

	fmt.Println("Recieved one message")

	assertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestDropUnmarshallableMessage(t *testing.T) {
	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- make([]byte, 10)

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}

func TestDropSinkWithoutApp(t *testing.T) {
	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8081", "/tail/")
	defer ws.Close()
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "mySpace", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}

func TestDropSinkWithoutAuthorization(t *testing.T) {
	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp")
	defer ws.Close()
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "mySpace", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}

func TestDropSinkWhenAuthorizationFails(t *testing.T) {
	t.Skipf("Skipp for now")
	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=incorrectAuthToken")
	defer ws.Close()
	// If you remove this line, the test will pass for the wrong reasons
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "mySpace", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}
