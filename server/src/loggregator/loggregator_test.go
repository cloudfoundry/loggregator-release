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

func TestEndtoEndMessage(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")
	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()
	messageRouter := sinkserver.NewMessageRouter(10, logger)
	httpServer := sinkserver.NewHttpServer(messageRouter, testhelpers.SuccessfulAuthorizer, 30*time.Second, logger)
	go messageRouter.Start()
	go httpServer.Start(dataChannel, "localhost:8081")
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)
	ws, _, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := messagetesthelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	messagetesthelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}
