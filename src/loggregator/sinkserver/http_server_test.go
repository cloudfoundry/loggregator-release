package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"loggregator/iprange"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestDumpMessagesFromChannelToWebsocketWhenEmpty(t *testing.T) {
	dumpChan := make(chan *logmessage.Message, 1)
	doneChan := make(chan bool)

	close(dumpChan)

	go func() {
		dumpMessagesFromChannelToWebsocket(dumpChan, nil, nil, loggertesthelper.Logger())
		close(doneChan)
	}()

	select {
	case <-doneChan:
		break
	case <-time.After(10 * time.Millisecond):
		t.FailNow()
	}

}

func TestConnectingToUnknownEndpointReturns400(t *testing.T) {
	AssertConnectionFails(t, SERVER_PORT, "/INVALID_PATH", 400)
}

func TestParseEnvelopesDoesntBlockWhenMessageRouterChannelIsFull(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")

	messageChannelLength := 1
	messageRouter := NewMessageRouter(1, true, []iprange.IPRange{}, logger, messageChannelLength)
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
