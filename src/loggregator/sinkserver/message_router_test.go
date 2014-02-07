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
)

type testSink struct {
	channel             chan *logmessage.Message
	shouldReceiveErrors bool
}

func (ts testSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

func (ts testSink) AppId() string {
	return "appId"
}

func (ts testSink) Run() {

}

func (ts testSink) ShouldReceiveErrors() bool {
	return ts.shouldReceiveErrors
}

func (ts testSink) Channel() chan *logmessage.Message {
	return ts.channel
}

func (ts testSink) Identifier() string {
	return "testSink"
}

func (ts testSink) Logger() *gosteno.Logger {
	return loggertesthelper.Logger()
}

// sink manager specific test
func TestErrorMessagesAreDeliveredToSinksThatSupportThem(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)
	sinkManager.errorChannel <- messagetesthelpers.NewMessage(t, "error msg", "appId")
	select {
	case errorMsg := <-ourSink.Channel():
		assert.Equal(t, string(errorMsg.GetLogMessage().GetMessage()), "error msg")
	case <-time.After(1 * time.Millisecond):
		t.Error("Should have received an error message")
	}
}

// sink manager specific test
func TestErrorMessagesAreNotDeliveredToSinksThatDontAcceptErrors(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)
	sinkManager.errorChannel <- messagetesthelpers.NewMessage(t, "error msg", "appId")
	select {
	case _ = <-ourSink.Channel():
		t.Error("Should not have received a message")
	case <-time.After(10 * time.Millisecond):
		break
	}
}

func xTestSendingToErrorChannelDoesNotBlock(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	sinkManager.errorChannel = make(chan *logmessage.Message, 1)
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	sinkChannel := make(chan *logmessage.Message, 10)
	ourSink := testSink{sinkChannel, false}

	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	for i := 0; i < 10; i++ {
		badMessage := messagetesthelpers.NewMessage(t, "error msg", "appIdWeDontCareAbout")
		badMessage.GetLogMessage().DrainUrls = []string{fmt.Sprintf("<nil%d>", i)}
		testMessageRouter.outgoingLogChan <- badMessage
	}

	goodMessage := messagetesthelpers.NewMessage(t, "error msg", "appId")
	testMessageRouter.outgoingLogChan <- goodMessage

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(1000 * time.Millisecond):
		t.Error("Should have received a message")
	}
}

func xTestThatItDoesNotCreateAnotherSyslogDrainIfItIsAlreadyThere(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.outgoingLogChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)

	testMessageRouter.outgoingLogChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)
}

func xTestSimpleBlacklistRule(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.outgoingLogChan <- message

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not message about blacklisted syslog drain")
	}

	testMessageRouter.outgoingLogChan <- message

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.Channel():
		t.Error("Should not receive another message about the blacklisted url since we cache blacklisted urls")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter)

	message = messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.2"}
	testMessageRouter.outgoingLogChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)
}

func xTestInvalidUrlForSyslogDrain(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	incomingLogChan := make(chan *logmessage.Message, 10)
	testMessageRouter := NewMessageRouter(incomingLogChan, sinkManager, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"ht tp://bad.protocol.com"}
	testMessageRouter.outgoingLogChan <- message

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not message about blacklisted syslog drain")
	}

	testMessageRouter.outgoingLogChan <- message

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(1000 * time.Millisecond):
		t.Error("Did not receive real message")
	}

	select {
	case _ = <-ourSink.Channel():
		t.Error("Should not receive another message about the blacklisted url since we cache blacklisted urls")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter)
}

func waitForMessageGettingProcessed(t *testing.T, ourSink testSink, timeout time.Duration) {
	select {
	case _ = <-ourSink.Channel():

	case <-time.After(timeout):
		t.Error("Message didn't get processed")
	}
}

func xTestParseEnvelopesDoesntBlockWhenMessageRouterChannelIsFull(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")

	sinkManager := NewSinkManager(1, true, []iprange.IPRange{}, logger)
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
