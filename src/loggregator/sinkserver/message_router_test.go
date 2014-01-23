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

func TestThatDumpingForAnAppWithoutADumpSinkDoesNotBlockForever(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	go testMessageRouter.Start()

	outputChannel := make(chan *logmessage.Message)
	dumpReceiver := dumpReceiver{outputChannel: outputChannel, appId: "doesNotExist"}
	testMessageRouter.dumpReceiverChan <- dumpReceiver

	select {
	case _, ok := <-outputChannel:
		if !ok {
			// All is good. Channel got closed
		} else {
			t.Error("Should not Receive anything")
		}
	case <-time.After(1 * time.Second):
		t.Error("Channel should be closed right away if there is no dump sink for the given app.")
	}
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

func TestSendingToErrorChannelDoesNotBlock(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	testMessageRouter.errorChannel = make(chan *logmessage.Message, 1)
	go testMessageRouter.Start()

	sinkChannel := make(chan *logmessage.Message, 10)
	ourSink := testSink{sinkChannel, false}

	testMessageRouter.sinkOpenChan <- ourSink
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
	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	testMessageRouter.sinkOpenChan <- ourSink
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

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter)

	message = messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"http://10.10.123.2"}
	testMessageRouter.parsedMessageChan <- message
	waitForMessageGettingProcessed(t, ourSink, 10*time.Millisecond)

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter+1)
}

func TestMisformattedSyslogDrain(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, loggertesthelper.Logger())
	oldActiveSyslogSinksCounter := testMessageRouter.activeSyslogSinksCounter

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	testMessageRouter.sinkOpenChan <- ourSink
	<-time.After(1 * time.Millisecond)

	message := messagetesthelpers.NewMessage(t, "error msg", "appId")
	message.GetLogMessage().DrainUrls = []string{"<nil>"}
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

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter)
}

func TestInvalidUrlForSyslogDrain(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, loggertesthelper.Logger())
	oldActiveSyslogSinksCounter := testMessageRouter.activeSyslogSinksCounter

	go testMessageRouter.Start()
	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	testMessageRouter.sinkOpenChan <- ourSink
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
	case readMessage := <-ourSink.Channel():
		println(string(readMessage.GetLogMessage().GetMessage()))
		t.Error("Should not receive another message about the blacklisted url since we cache blacklisted urls")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Equal(t, testMessageRouter.activeSyslogSinksCounter, oldActiveSyslogSinksCounter)
}

func TestStopsRetryingWhenSinkIsUnregistered(t *testing.T) {
	testMessageRouter := NewMessageRouter(1024, false, []iprange.IPRange{iprange.IPRange{Start: "10.10.123.1", End: "10.10.123.1"}}, loggertesthelper.Logger())
	go testMessageRouter.Start()

	ourSink := testSink{make(chan *logmessage.Message, 100), true}
	testMessageRouter.sinkOpenChan <- ourSink
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
