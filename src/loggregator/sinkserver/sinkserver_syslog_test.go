package sinkserver

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"regexp"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestThatItSendsAllMessageToKnownDrains(t *testing.T) {
	client1ReceivedChan := make(chan []byte)

	fakeSyslogDrain, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34566")
	defer fakeSyslogDrain.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString := "Some Data"
	logEnvelope1 := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp", SECRET, "syslog://localhost:34566")

	expectedSecondMessageString := "Some Data Without a drainurl"
	logEnvelope2 := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedSecondMessageString, "myApp", SECRET, "syslog://localhost:34566")

	dataReadChannel <- logEnvelope1

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get the first message")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	dataReadChannel <- logEnvelope2

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get the second message")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedSecondMessageString)
	}
}

func TestThatItReestablishesConnectionToSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)

	assert.Equal(t, len(dataReadChannel), 0)

	fakeSyslogDrain, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString1 := "Some Data 1"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString1, "myApp", SECRET, "syslog://localhost:34569")
	dataReadChannel <- logEnvelope

	errorString := "Did not get the first message. Server was up, it should have been there"
	assertMessageOnChannel(t, 200, client1ReceivedChan, errorString, expectedMessageString1)
	fakeSyslogDrain.Stop()

	expectedMessageString2 := "Some Data 2"
	logEnvelope = messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString2, "myApp", SECRET, "syslog://localhost:34569")
	dataReadChannel <- logEnvelope
	error2String := "Did get a second message! Shouldn't be since the server is down"
	assertMessageNotOnChannel(t, 200, client1ReceivedChan, error2String)

	expectedMessageString3 := "Some Data 3"
	logEnvelope = messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString3, "myApp", SECRET, "syslog://localhost:34569")
	dataReadChannel <- logEnvelope
	error3String := "Did get a third message! Shouldn't be since the server is down"
	assertMessageNotOnChannel(t, 200, client1ReceivedChan, error3String)

	client2ReceivedChan := make(chan []byte, 10)
	fakeSyslogDrain, err = NewFakeService(client2ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)

	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString4 := "Some Data 4"
	logEnvelope = messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString4, "myApp", SECRET, "syslog://localhost:34569")
	dataReadChannel <- logEnvelope

	error4String := "Did not get the fourth message, but it should have been just fine since the server was up"
	assertMessageOnChannel(t, 200, client2ReceivedChan, error4String, expectedMessageString4)
	fakeSyslogDrain.Stop()
}

func TestThatItSendsAllDataToAllDrainUrls(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34567")
	assert.NoError(t, err)
	go fakeSyslogDrain1.Serve()
	<-fakeSyslogDrain1.ReadyChan

	fakeSyslogDrain2, err := NewFakeService(client2ReceivedChan, "127.0.0.1:34568")
	assert.NoError(t, err)
	go fakeSyslogDrain2.Serve()
	<-fakeSyslogDrain2.ReadyChan

	expectedMessageString := "Some Data"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp", SECRET, "syslog://localhost:34567", "syslog://localhost:34568")
	dataReadChannel <- logEnvelope

	errString := "Did not get message from client 1."
	assertMessageOnChannel(t, 500, client1ReceivedChan, errString, expectedMessageString)

	errString = "Did not get message from client 2."
	assertMessageOnChannel(t, 500, client2ReceivedChan, errString, expectedMessageString)

	fakeSyslogDrain1.Stop()
	fakeSyslogDrain2.Stop()
}

// this test fails intermittently
func TestThatItSendsAllDataToOnlyAuthoritiveMessagesWithDrainUrls(t *testing.T) {
	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	fakeSyslogDrain1, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	defer fakeSyslogDrain1.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain1.Serve()
	<-fakeSyslogDrain1.ReadyChan

	fakeSyslogDrain2, err := NewFakeService(client2ReceivedChan, "127.0.0.1:34540")
	defer fakeSyslogDrain2.Stop()
	assert.NoError(t, err)
	go fakeSyslogDrain2.Serve()
	<-fakeSyslogDrain2.ReadyChan

	expectedMessageString := "Some Data"
	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, expectedMessageString, "myApp", SECRET, "syslog://localhost:34569")
	dataReadChannel <- logEnvelope

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
	logEnvelope2 := messagetesthelpers.MarshalledLogEnvelope(t, logMessage2, SECRET)

	dataReadChannel <- logEnvelope2

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 2")
	case message := <-client1ReceivedChan:
		matched, _ := regexp.MatchString(`<14>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}([-+]\d{2}:\d{2}) loggregator myApp DEA - - loggregator myApp: loggregator myApp: Some More Data`, string(message))
		assert.True(t, matched, string(message))
	case <-client2ReceivedChan:
		t.Error("Should not have gotten the new message in this drain")
	}
}

func TestThatItDoesNotRegisterADrainIfItsURLIsBlacklisted(t *testing.T) {
	receivedChan := make(chan []byte, 2)
	testhelpers.AddWSSink(t, receivedChan, BLACKLIST_SERVER_PORT, TAIL_PATH+"?app=myApp01")
	WaitForWebsocketRegistration()

	clientReceivedChan := make(chan []byte)

	blackListedSyslogDrain, err := NewFakeService(clientReceivedChan, "127.0.0.1:34570")
	defer blackListedSyslogDrain.Stop()
	assert.NoError(t, err)
	go blackListedSyslogDrain.Serve()
	<-blackListedSyslogDrain.ReadyChan

	logEnvelope1 := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, "Some Data", "myApp01", SECRET, "syslog://127.0.0.1:34570")
	blackListDataReadChannel <- logEnvelope1

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
