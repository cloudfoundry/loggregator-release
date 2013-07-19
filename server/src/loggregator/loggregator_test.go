package loggregator

import (
	"github.com/stretchr/testify/assert"
	"github.com/cloudfoundry/gosteno"
	"code.google.com/p/gogoprotobuf/proto"
	"testing"
	"loggregator/agentlistener"
	"loggregator/cfsink"
	"net"
	"time"
	"code.google.com/p/go.net/websocket"
	"logMessage"
)

func TestEndtoEndMessage(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")
	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()
	sinkServer := cfsink.NewCfSinkServer(dataChannel, logger, "localhost:8081", "/tail/", "http://localhost:9876", cfSink.SuccessfulAuthorizer)
	go sinkServer.Start()
	time.Sleep(1*time.Millisecond)

	receivedChan := make(chan []byte)
	ws := cfsink.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	cfsink.WaitForWebsocketRegistration()

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := cfsink.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	cfsink.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}
