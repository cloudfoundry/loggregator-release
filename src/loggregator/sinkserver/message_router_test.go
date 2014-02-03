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
	"runtime"
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

func TestErrorMessagesAreDeliveredToSinksThatSupportThem(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
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

func TestErrorMessagesAreNotDeliveredToSinksThatDontAcceptErrors(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
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

func TestSendingToErrorChannelDoesNotBlock(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	sinkManager.errorChannel = make(chan *logmessage.Message, 1)
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
	go testMessageRouter.Start()

	sinkChannel := make(chan *logmessage.Message, 10)
	ourSink := testSink{sinkChannel, false}

	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	for i := 0; i < 10; i++ {
		badMessage := messagetesthelpers.NewMessage(t, "error msg", "appIdWeDontCareAbout")
		badMessage.GetLogMessage().DrainUrls = []string{fmt.Sprintf("<nil%d>", i)}
		testMessageRouter.parsedMessageChan <- badMessage
	}

	goodMessage := messagetesthelpers.NewMessage(t, "error msg", "appId")
	testMessageRouter.parsedMessageChan <- goodMessage

	select {
	case _ = <-ourSink.Channel():
	case <-time.After(1000 * time.Millisecond):
		t.Error("Should have received a message")
	}
}

func TestThatItDoesNotCreateAnotherSyslogDrainIfItIsAlreadyThere(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, nil, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)

	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)
}

func TestSimpleBlacklistRule(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.parsedMessageChan <- message

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

	testMessageRouter.parsedMessageChan <- message

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
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, sinkManager.Metrics.SyslogSinks, oldActiveSyslogSinksCounter+1)
}

func TestInvalidUrlForSyslogDrain(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	oldActiveSyslogSinksCounter := sinkManager.Metrics.SyslogSinks
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"ht tp://bad.protocol.com"}
	testMessageRouter.parsedMessageChan <- message

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

	testMessageRouter.parsedMessageChan <- message

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

func TestStopsRetryingWhenSinkIsUnregistered(t *testing.T) {
	logger := loggertesthelper.Logger()
	sinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, logger)
	go sinkManager.Start()

	testMessageRouter := NewMessageRouter(sinkManager, 2048, logger)
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	sinkManager.sinkOpenChan <- ourSink
	runtime.Gosched()

	go func() {
		message := messagetesthelpers.NewMessage(t, "error msg", "appId")
		message.GetLogMessage().DrainUrls = []string{"syslog://localhost:41223"}
		testMessageRouter.parsedMessageChan <- message

		newMessage := messagetesthelpers.NewMessage(t, "RemoveSyslogSink", "appId")
		testMessageRouter.parsedMessageChan <- newMessage

	}()

	for {
		readMessage := <-ourSink.Channel()
		if string(readMessage.GetLogMessage().GetMessage()) == "RemoveSyslogSink" {
			break
		}
	}

	select {
	case message := <-ourSink.Channel():
		t.Errorf("Should not receive another message after removal; message was %v", string(message.GetLogMessage().GetMessage()))
	case <-time.After(2 * time.Second):
	}
}

func waitForMessageGettingProcessed(t *testing.T, ourSink testSink, timeout time.Duration) {
	select {
	case _ = <-ourSink.Channel():

	case <-time.After(timeout):
		t.Error("Message didn't get processed")
	}
}
