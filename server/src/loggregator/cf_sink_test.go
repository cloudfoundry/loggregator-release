package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"net/http"
	"testing"
	"time"
)

func init() {
	startFakeCloudController := func() {
		handleCloudControllerRequest := func(w http.ResponseWriter, r *http.Request) {
			if r.Header["Authorization"][0] != "bearer correctAuthorizationToken" {
				w.WriteHeader(401)
			}
		}
		http.HandleFunc("/v2/apps/", handleCloudControllerRequest)
		http.ListenAndServe(":9876", nil)
	}

	go startFakeCloudController()
}

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

func marshalledLogMessage(t *testing.T, messageString string, appId string) []byte {
	currentTime := time.Now()

	messageType := logMessage.LogMessage_OUT
	sourceType := logMessage.LogMessage_DEA
	protoMessage := &logMessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		SpaceId:     proto.String(appId + "Space"),
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

func TestThatItSends(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8081", "/tail/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte, 2)

	expectedMessageString := "Some Data"
	expectedMessage := marshalledLogMessage(t, expectedMessageString, "myApp")
	otherMessageString := "More stuff"
	otherMessage := marshalledLogMessage(t, otherMessageString, "myApp")

	ws := addWSSink(t, receivedChan, "8081", "/tail/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- expectedMessage
	dataReadChannel <- otherMessage

	assertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
	assertProtoBufferMessageEquals(t, otherMessageString, <-receivedChan)
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8082", "/tail2/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	expectedMessageString := "Some Data"
	expectedMarshalledProtoBuffer := marshalledLogMessage(t, expectedMessageString, "myApp")

	wsClient1 := addWSSink(t, client1ReceivedChan, "8082", "/tail2/myApp?authorization=bearer%20correctAuthorizationToken")
	defer wsClient1.Close()

	wsClient2 := addWSSink(t, client2ReceivedChan, "8082", "/tail2/myApp?authorization=bearer%20correctAuthorizationToken")
	defer wsClient2.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- expectedMarshalledProtoBuffer

	assertProtoBufferMessageEquals(t, expectedMessageString, <-client1ReceivedChan)
	assertProtoBufferMessageEquals(t, expectedMessageString, <-client2ReceivedChan)
}

func TestThatItSendsLogsForOneApplication(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8083", "/tail3/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte, 2)

	otherAppsMarshalledMessage := marshalledLogMessage(t, "Some other message", "otherApp")
	expectedMessageString := "My important message"
	myAppsMarshalledMessage := marshalledLogMessage(t, expectedMessageString, "myApp")

	ws := addWSSink(t, receivedChan, "8083", "/tail3/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	waitForWebsocketRegistration()

	dataReadChannel <- otherAppsMarshalledMessage
	dataReadChannel <- myAppsMarshalledMessage

	assertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestDropUnmarshallableMessage(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8084", "/tail4/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8084", "/tail4/myApp?authorization=bearer%20correctAuthorizationToken")
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
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8085", "/tail5/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8085", "/tail5/")
	defer ws.Close()
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "myApp")
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
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8086", "/tail6/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8086", "/tail6/myApp")
	defer ws.Close()
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}

func TestDropSinkWithIncorrectAuthorization(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8087", "/tail7/", "http://localhost:9876")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)

	ws := addWSSink(t, receivedChan, "8087", "/tail7/myApp?authorization=incorrectAuthToken")
	defer ws.Close()
	// If you remove this line, the test will pass for the wrong reasons
	waitForWebsocketRegistration()

	myAppsMarshalledMessage := marshalledLogMessage(t, "I won't make it..", "myApp")
	dataReadChannel <- myAppsMarshalledMessage

	time.Sleep(1 * time.Millisecond)
	select {
	case msg1 := <-receivedChan:
		t.Error("We should not have received a message, but got: %v", msg1)
	default:
		//no communication. That's good!
	}
}
