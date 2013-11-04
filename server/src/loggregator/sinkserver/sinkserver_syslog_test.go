package sinkserver

import (
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
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34566")

	expectedSecondMessageString := "Some Data Without a drainurl"
	expectedSecondMarshalledProtoBuffer := messagetesthelpers.MarshalledLogMessage(t, expectedSecondMessageString, "myApp")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get the first message")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	dataReadChannel <- expectedSecondMarshalledProtoBuffer

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
	expectedMarshalledProtoBuffer1 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString1, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer1

	errorString := "Did not get the first message. Server was up, it should have been there"
	AssertMessageOnChannel(t, 200, client1ReceivedChan, errorString, expectedMessageString1)
	fakeSyslogDrain.Stop()
	time.Sleep(50 * time.Millisecond)

	expectedMessageString2 := "Some Data 2"
	expectedMarshalledProtoBuffer2 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString2, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer2
	errorString = "Did get a second message! Shouldn't be since the server is down"
	AssertMessageNotOnChannel(t, 200, client1ReceivedChan, errorString)

	expectedMessageString3 := "Some Data 3"
	expectedMarshalledProtoBuffer3 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString3, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer3
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
	expectedMarshalledProtoBuffer4 := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString4, "myApp", "syslog://localhost:34569")
	dataReadChannel <- expectedMarshalledProtoBuffer4

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
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34567", "syslog://localhost:34568")

	dataReadChannel <- expectedMarshalledProtoBuffer

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
	expectedMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedLogMessage(t, expectedMessageString, "myApp", "syslog://localhost:34569")

	dataReadChannel <- expectedMarshalledProtoBuffer

	select {
	case <-time.After(200 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case message := <-client1ReceivedChan:
		assert.Contains(t, string(message), expectedMessageString)
	}

	expectedSecondMessageString := "loggregator myApp: loggregator myApp: Some More Data"
	expectedSecondMarshalledProtoBuffer := messagetesthelpers.MarshalledDrainedNonWardenLogMessage(t, expectedSecondMessageString, "myApp", "syslog://localhost:34540")

	dataReadChannel <- expectedSecondMarshalledProtoBuffer

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
