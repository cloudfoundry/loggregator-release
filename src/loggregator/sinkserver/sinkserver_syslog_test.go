package sinkserver_test

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregator/sinkserver"
	"regexp"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

// this test fails intermittently
func TestThatItSendsAllDataToOnlyAuthoritiveMessagesWithDrainUrls(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)
	fakeSyslogDrain1.Serve()
	defer fakeSyslogDrain1.Stop()
	<-fakeSyslogDrain1.ReadyChan

	fakeSyslogDrain2, err := NewFakeService(client2ReceivedChan, "127.0.0.1:34540")
	assert.NoError(t, err)
	fakeSyslogDrain2.Serve()
	defer fakeSyslogDrain2.Stop()
	<-fakeSyslogDrain2.ReadyChan

	expectedMessageString := "Some Data"
	message := messagetesthelpers.NewMessageWithSyslogDrain(t, expectedMessageString, "myApp", "syslog://localhost:34569")
	dataReadChannel <- message

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	expectedSecondMessageString := "loggregator myApp: loggregator myApp: Some More Data"
	logMessage2 := messagetesthelpers.NewLogMessage(expectedSecondMessageString, "myApp")
	sourceName := "DEA"
	logMessage2.SourceName = &sourceName
	logMessage2.DrainUrls = []string{"syslog://localhost:34540"}
	message2 := messagetesthelpers.NewMessageFromLogMessage(t, logMessage2)

	dataReadChannel <- message2

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 2")
	case message := <-client1ReceivedChan:
		matched, _ := regexp.MatchString(`<14>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}([-+]\d{2}:\d{2}) loggregator myApp \[DEA\] - - loggregator myApp: loggregator myApp: Some More Data`, string(message))
		assert.True(t, matched, string(message))
	case <-client2ReceivedChan:
		t.Error("Should not have gotten the new message in this drain")
	}
}

func TestThatItDoesNotRegisterADrainIfItsURLIsBlacklisted(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	testhelpers.AddWSSink(t, receivedChan, BLACKLIST_SERVER_PORT, sinkserver.TAIL_LOGS_PATH+"?app=myApp01")
	WaitForWebsocketRegistration()

	clientReceivedChan := make(chan []byte)

	blackListedSyslogDrain, err := NewFakeService(clientReceivedChan, "127.0.0.1:34570")
	defer blackListedSyslogDrain.Stop()
	assert.NoError(t, err)
	blackListedSyslogDrain.Serve()
	<-blackListedSyslogDrain.ReadyChan

	message1 := messagetesthelpers.NewMessageWithSyslogDrain(t, "Some Data", "myApp01", "syslog://127.0.0.1:34570")
	blackListDataReadChannel <- message1

	assertMessageNotOnChannel(t, 1000, clientReceivedChan, "Should not have received message on blacklisted Syslog drain")

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get the real message.")
	case <-receivedChan:
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Did not get the error message about the blacklisted syslog drain.")
	case receivedMessage := <-receivedChan:
		messagetesthelpers.AssertProtoBufferMessageContains(t, "URL is blacklisted", receivedMessage)
	}
}

func assertMessageOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string, expectedMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		t.Errorf("%s", errorMessage)
	case message := <-receiveChan:
		assert.Contains(t, string(message), expectedMessage)
	}
}

func assertMessageNotOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	case message := <-receiveChan:
		t.Errorf("%s %s", errorMessage, string(message))
	}
}
