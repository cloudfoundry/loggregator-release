package sinkserver

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"regexp"
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
	logMessage1 := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")
	logMessage1.DrainUrls = []string{"syslog://localhost:34566"}
	logEnvelope1 := messagetesthelpers.MarshalledLogEnvelope(t, logMessage1, SECRET)

	expectedSecondMessageString := "Some Data Without a drainurl"
	logMessage2 := messagetesthelpers.NewLogMessage(expectedSecondMessageString, "myApp")
	logMessage2.DrainUrls = []string{"syslog://localhost:34566"}
	logEnvelope2 := messagetesthelpers.MarshalledLogEnvelope(t, logMessage2, SECRET)

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

func AssertMessageOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string, expectedMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		t.Errorf("%s", errorMessage)
	case message := <-receiveChan:
		assert.Contains(t, string(message), expectedMessage)
	}
}

func AssertMessageNotOnChannel(t *testing.T, timeout int, receiveChan chan []byte, errorMessage string) {
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	case message := <-receiveChan:
		t.Errorf("%s %s", errorMessage, string(message))
	}
}

func TestThatItReestablishesConnectionToSinks(t *testing.T) {
	client1ReceivedChan := make(chan []byte)

	fakeSyslogDrain, err := NewFakeService(client1ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	expectedMessageString1 := "Some Data 1"
	logMessage := messagetesthelpers.NewLogMessage(expectedMessageString1, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34569"}
	logEnvelope := messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)
	dataReadChannel <- logEnvelope

	errorString := "Did not get the first message. Server was up, it should have been there"
	AssertMessageOnChannel(t, 200, client1ReceivedChan, errorString, expectedMessageString1)
	fakeSyslogDrain.Stop()
	time.Sleep(50 * time.Millisecond)

	expectedMessageString2 := "Some Data 2"
	logMessage = messagetesthelpers.NewLogMessage(expectedMessageString2, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34569"}
	logEnvelope = messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)
	dataReadChannel <- logEnvelope
	errorString = "Did get a second message! Shouldn't be since the server is down"
	AssertMessageNotOnChannel(t, 200, client1ReceivedChan, errorString)

	expectedMessageString3 := "Some Data 3"
	logMessage = messagetesthelpers.NewLogMessage(expectedMessageString3, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34569"}
	logEnvelope = messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)
	dataReadChannel <- logEnvelope
	errorString = "Did get a third message! Shouldn't be since the server is down"
	AssertMessageNotOnChannel(t, 200, client1ReceivedChan, errorString)

	time.Sleep(2260 * time.Millisecond)

	client2ReceivedChan := make(chan []byte, 10)
	fakeSyslogDrain, err = NewFakeService(client2ReceivedChan, "127.0.0.1:34569")
	assert.NoError(t, err)

	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	time.Sleep(2260 * time.Millisecond)

	expectedMessageString4 := "Some Data 4"
	logMessage = messagetesthelpers.NewLogMessage(expectedMessageString4, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34569"}
	logEnvelope = messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)
	dataReadChannel <- logEnvelope

	errorString = "Did not get the fourth message, but it should have been just fine since the server was up"
	AssertMessageOnChannel(t, 200, client2ReceivedChan, errorString, expectedMessageString4)

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
	logMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34567", "syslog://localhost:34568"}
	logEnvelope := messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)

	dataReadChannel <- logEnvelope

	errString := "Did not get message from client 1."
	AssertMessageOnChannel(t, 200, client1ReceivedChan, errString, expectedMessageString)

	errString = "Did not get message from client 2."
	AssertMessageOnChannel(t, 200, client2ReceivedChan, errString, expectedMessageString)

	fakeSyslogDrain1.Stop()
	fakeSyslogDrain2.Stop()
}

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
	logMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")
	logMessage.DrainUrls = []string{"syslog://localhost:34569"}
	logEnvelope := messagetesthelpers.MarshalledLogEnvelope(t, logMessage, SECRET)

	dataReadChannel <- logEnvelope

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	expectedSecondMessageString := "loggregator myApp: loggregator myApp: Some More Data"
	logMessage2 := messagetesthelpers.NewLogMessage(expectedSecondMessageString, "myApp")
	sourceType := logmessage.LogMessage_DEA
	logMessage2.SourceType = &sourceType
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
