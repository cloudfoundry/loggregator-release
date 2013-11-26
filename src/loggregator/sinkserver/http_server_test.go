package sinkserver

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
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
