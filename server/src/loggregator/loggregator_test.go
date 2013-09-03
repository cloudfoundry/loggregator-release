package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/stretchr/testify/assert"
	"loggregator/messagestore"
	"loggregator/sink"
	"net"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestEndtoEndMessage(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")
	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()
	sinkServer := sink.NewSinkServer(dataChannel, messagestore.NewMessageStore(10), logger, "localhost:8081", testhelpers.SuccessfulAuthorizer, 30*time.Second)
	go sinkServer.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)
	ws, _, _ := testhelpers.AddWSSink(t, receivedChan, "8081", "/tail/?app=myApp", testhelpers.VALID_SPACE_AUTHENTICATION_TOKEN)
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := testhelpers.MarshalledLogMessage(t, expectedMessageString, "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	testhelpers.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}
