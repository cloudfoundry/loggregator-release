package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
)

func TestThatItWorksLikeAChannel(t *testing.T) {
	inMessageChan := make(chan *logmessage.Message)
	outMessageChan := make(chan *logmessage.Message, 2)
	go RingBufferChannel(inMessageChan, outMessageChan, nil)

	logMessage1, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "message 1", "appId"))
	assert.NoError(t, err)
	inMessageChan <- logMessage1
	readMessage := <-outMessageChan
	assert.Contains(t, string(readMessage.GetRawMessage()), "message 1")

	logMessage2, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "message 2", "appId"))
	assert.NoError(t, err)
	inMessageChan <- logMessage2
	readMessage2 := <-outMessageChan
	assert.Contains(t, string(readMessage2.GetRawMessage()), "message 2")

}

func TestThatItWorksLikeABufferedRingChannel(t *testing.T) {
	inMessageChan := make(chan *logmessage.Message)
	outMessageChan := make(chan *logmessage.Message, 2)
	go RingBufferChannel(inMessageChan, outMessageChan, nil)

	logMessage1, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "message 1", "appId"))
	assert.NoError(t, err)
	inMessageChan <- logMessage1

	logMessage2, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "message 2", "appId"))
	assert.NoError(t, err)
	inMessageChan <- logMessage2

	logMessage3, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "message 3", "appId"))
	assert.NoError(t, err)
	inMessageChan <- logMessage3

	readMessage := <-outMessageChan
	assert.Contains(t, string(readMessage.GetRawMessage()), "message 2")

	readMessage2 := <-outMessageChan
	assert.Contains(t, string(readMessage2.GetRawMessage()), "message 3")

}
