package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregator/sinkserver"
	"net"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func init() {
	logger := gosteno.NewLogger("TestLogger")

	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()

	messageRouter := sinkserver.NewMessageRouter(10, false, nil, logger)
	go messageRouter.Start()
	httpServer := sinkserver.NewHttpServer(messageRouter, 30*time.Second, testhelpers.UnmarshallerMaker("secret"), logger)
	go httpServer.Start(dataChannel, "localhost:8081")

	time.Sleep(50 * time.Millisecond)
}

func TestEndtoEndMessageShouldNotWork(t *testing.T) {
	receivedChan := make(chan []byte)
	ws, _, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/?app=myApp")
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	select {
	case _ = <-receivedChan:
		t.Error("Message should have been dropped")
	case <-time.After(2 * time.Second):
		//success
	}
}

func TestEndtoEndEnvelopeToMessage(t *testing.T) {
	receivedChan := make(chan []byte)
	ws, _, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/?app=myApp")
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	unmarshalledLogMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")

	expectedMessage := messagetesthelpers.MarshalledLogEnvelope(t, unmarshalledLogMessage, "secret")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}

func TestInvalidEnvelopeEndtoEnd(t *testing.T) {
	receivedChan := make(chan []byte)
	ws, _, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/?app=myApp2")
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	unmarshalledLogMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp2")

	expectedMessage := messagetesthelpers.MarshalledLogEnvelope(t, unmarshalledLogMessage, "invalid")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	select {
	case _ = <-receivedChan:
		t.Error("Message should have been dropped")
	case <-time.After(2 * time.Second):
		//success
	}
}
