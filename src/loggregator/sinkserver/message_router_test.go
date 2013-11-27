package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregator/groupedsinks"
	"loggregator/iprange"
	"loggregator/sinks"
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

func TestDumpToSinkWithLessThan20Messages(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())

	activeSinks := groupedsinks.NewGroupedSinks()
	dumpSink := sinks.NewDumpSink("appId", 100, loggertesthelper.Logger())
	activeSinks.Register(dumpSink)

	message := messagetesthelpers.NewMessage(t, "message 1", "appId")
	for i := 0; i < 19; i++ {
		dumpSink.Channel() <- message
	}
	close(dumpSink.Channel())
	<-time.After(10 * time.Millisecond)

	sink := testSink{make(chan *logmessage.Message, 100), false}
	testMessageRouter.dumpToSink(sink, activeSinks)

	assert.Equal(t, 19, len(sink.Channel()))
}

func TestDumpToSinkLimitsMessagesTo20(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	sink := testSink{make(chan *logmessage.Message, 100), false}
	activeSinks := groupedsinks.NewGroupedSinks()
	dumpSink := sinks.NewDumpSink("appId", 100, loggertesthelper.Logger())

	message := messagetesthelpers.NewMessage(t, "message 1", "appId")
	for i := 0; i < 100; i++ {
		dumpSink.Channel() <- message
	}

	activeSinks.Register(dumpSink)
	testMessageRouter.dumpToSink(sink, activeSinks)

	assert.Equal(t, 20, len(sink.Channel()))
}

func TestErrorMessagesAreDeliveredToSinksThatSupportThem(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	testMessageRouter.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)
	testMessageRouter.errorChannel <- messagetesthelpers.NewMessage(t, "error msg", "appId")
	select {
	case errorMsg := <-ourSink.Channel():
		assert.Equal(t, string(errorMsg.GetLogMessage().GetMessage()), "error msg")
	case <-time.After(1 * time.Millisecond):
		t.Error("Should have received an error message")
	}
}

func TestErrorMessagesAreNotDeliveredToSinksThatDontAcceptErrors(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	testMessageRouter.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)
	testMessageRouter.errorChannel <- messagetesthelpers.NewMessage(t, "error msg", "appId")
	select {
	case _ = <-ourSink.Channel():
		t.Error("Should not have received a message")
	case <-time.After(10 * time.Millisecond):
		break
	}
}

func TestThatItDoesNotCreateAnotherSyslogDrainIfItIsAlreadyThere(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	oldActiveSyslogSinksCounter := testMessageRouter.activeSyslogSinksCounter

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	testMessageRouter.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter+1)

	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter+1)
}

func TestSimpleBlacklistRule(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, loggertesthelper.Logger())
	oldActiveSyslogSinksCounter := testMessageRouter.activeSyslogSinksCounter

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), false}
	testMessageRouter.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.1"}
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter)

	message = messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.2"}
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter+1)
}

func waitForMessageGettingProcessed(t *testing.T, ourSink testSink, timeout time.Duration) {
	select {
	case _ = <-ourSink.Channel():

	case <-time.After(timeout):
		t.Error("Message didn't get processed")
	}
}
