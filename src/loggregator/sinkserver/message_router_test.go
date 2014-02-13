package sinkserver

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregator/iprange"
	"testing"
	"time"
	"loggregator/sinkserver/sinkmanager"
)

type testSink struct {
	received            chan *logmessage.Message
	shouldReceiveErrors bool
}

func (ts testSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

func (ts testSink) AppId() string {
	return "appId"
}

func (ts testSink) Run(in <-chan *logmessage.Message) {
	for msg := range(in) {
		ts.received <- msg
	}
}

func (ts testSink) ShouldReceiveErrors() bool {
	return ts.shouldReceiveErrors
}

func (ts testSink) Identifier() string {
	return "testSink"
}

// TODO: move to sink manager
func TestSendingToErrorChannelDoesNotBlock(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := sinkmanager.NewSinkManager(1024, false, nil, logger)
	//sinkManager.errorChannel = make(chan *logmessage.Message, 1)
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 10), false}

	sinkManager.RegisterSink(ourSink)

	for i := 0; i < 10; i++ {
		badMessage := messagetesthelpers.NewMessage(t, "error msg", "appIdWeDontCareAbout")
		badMessage.GetLogMessage().DrainUrls = []string{fmt.Sprintf("<nil%d>", i)}
		incomingLogChan <- badMessage
	}

	goodMessage := messagetesthelpers.NewMessage(t, "error msg", "appId")
	incomingLogChan <- goodMessage

	select {
	case <- ourSink.received:
	case <-time.After(1000 * time.Millisecond):
		t.Error("Should have received a message")
	}
}

func TestSimpleBlacklistRule(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := sinkmanager.NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.RegisterSink(ourSink)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	incomingLogChan <- message

	select {
	case _ = <-ourSink.received:
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.received:
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not message about blacklisted syslog drain")
	}

	incomingLogChan <- message

	select {
	case _ = <-ourSink.received:
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.received:
		t.Error("Should not receive another message about the blacklisted url since we cache blacklisted urls")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter)

	message = messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.2"}
	incomingLogChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)
}

func TestInvalidUrlForSyslogDrain(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := sinkmanager.NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.RegisterSink(ourSink)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"ht tp://bad.protocol.com"}
	incomingLogChan <- message

	select {
	case _ = <-ourSink.received:
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.received:
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not message about blacklisted syslog drain")
	}

	incomingLogChan <- message

	select {
	case _ = <-ourSink.received:
	case <-time.After(1000 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.received:
		t.Error("Should not receive another message about the blacklisted url since we cache blacklisted urls")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter)
}

func waitForMessageGettingProcessed(t *testing.T, ourSink testSink, timeout time.Duration) {
	select {
	case _ = <-ourSink.received:

	case <-time.After(timeout):
		t.Error("Message didn't get processed")
	}
}

func TestParseEnvelopesDoesntBlockWhenMessageRouterChannelIsFull(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")

	sinkManager := sinkmanager.NewSinkManager(1, true, []iprange.IPRange{}, logger)
	go sinkManager.Start()
	incomingLogChan := make(chan *logmessage.Message, 1)
	messageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go messageRouter.Start()

	testMessage := messagetesthelpers.NewMessage(t, "msg", "appid")

	for i := 0; i < 10; i++ {
		select {
		case incomingLogChan <- testMessage:
			break
		case <-time.After(1 * time.Second):
			t.Fatal("Shouldn't have blocked")
		}
	}
}
