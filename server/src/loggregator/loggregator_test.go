package loggregator

import (
	"github.com/stretchr/testify/assert"
	"github.com/cloudfoundry/gosteno"
	"code.google.com/p/gogoprotobuf/proto"
	"testing"
	"loggregator/agentlistener"
	"loggregator/sink"
	"net"
	"time"
	"code.google.com/p/go.net/websocket"
	"logMessage"
)

func TestEndtoEndMessage(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")
	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()
	sinkServer := sink.NewCfSinkServer(dataChannel, logger, "localhost:8081", "/tail/", "http://localhost:9876", sink.SuccessfulAuthorizer)
	go sinkServer.Start()
	time.Sleep(1*time.Millisecond)

	receivedChan := make(chan []byte)
	ws := sink.AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	sink.WaitForWebsocketRegistration()

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := sink.MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	sink.AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}
