package truncatingbuffer

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestThatItWorksLikeAChannel(t *testing.T) {
	inMessageChan := make(chan *logmessage.Message)
	buffer := NewTruncatingBuffer(inMessageChan, 2, nil)
	go buffer.Run()

	logMessage1 := messagetesthelpers.NewMessage(t, "message 1", "appId")
	inMessageChan <- logMessage1
	readMessage := <-buffer.GetOutputChannel()
	assert.Contains(t, string(readMessage.GetRawMessage()), "message 1")

	logMessage2 := messagetesthelpers.NewMessage(t, "message 2", "appId")
	inMessageChan <- logMessage2
	readMessage2 := <-buffer.GetOutputChannel()
	assert.Contains(t, string(readMessage2.GetRawMessage()), "message 2")
}

func TestThatItWorksLikeATruncatingChannel(t *testing.T) {
	inMessageChan := make(chan *logmessage.Message)
	buffer := NewTruncatingBuffer(inMessageChan, 2, nil)
	go buffer.Run()

	logMessage1 := messagetesthelpers.NewMessage(t, "message 1", "appId")
	inMessageChan <- logMessage1

	logMessage2 := messagetesthelpers.NewMessage(t, "message 2", "appId")
	inMessageChan <- logMessage2

	logMessage3 := messagetesthelpers.NewMessage(t, "message 3", "appId")
	inMessageChan <- logMessage3
	time.Sleep(5 + time.Millisecond)

	readMessage := <-buffer.GetOutputChannel()
	assert.Contains(t, string(readMessage.GetRawMessage()), "We've truncated 2 messages")

	readMessage2 := <-buffer.GetOutputChannel()
	assert.Contains(t, string(readMessage2.GetRawMessage()), "message 3")

}
