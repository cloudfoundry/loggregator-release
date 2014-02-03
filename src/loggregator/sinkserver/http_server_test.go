package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"loggregator/iprange"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestConnectingToUnknownEndpointReturns400(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, "/INVALID_PATH", 400)
}

func TestParseEnvelopesDoesntBlockWhenMessageRouterChannelIsFull(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")

	messageChannelLength := 1
	sinkManager := NewSinkManager(1, true, []iprange.IPRange{}, logger)
	go sinkManager.Start()
	messageRouter := NewMessageRouter(sinkManager, messageChannelLength, logger)
	httpServer := NewHttpServer(messageRouter, 30*time.Second, testhelpers.UnmarshallerMaker("secret"), 10, logger)
	incomingProtobufChan := make(chan []byte, 1)
	go httpServer.ParseEnvelopes(incomingProtobufChan)

	testMessage := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, "msg", "appName", "secret")

	for i := 0; i < 10; i++ {
		select {
		case incomingProtobufChan <- testMessage:
			break
		case <-time.After(1 * time.Second):
			t.Fatal("Shouldn't have blocked")
		}
	}
}
