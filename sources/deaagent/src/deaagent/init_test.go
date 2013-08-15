package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"logmessage"
	"testing"
)

func getBackendMessage(t *testing.T, data *[]byte) *logmessage.LogMessage {
	receivedMessage := &logmessage.LogMessage{}

	err := proto.Unmarshal(*data, receivedMessage)

	if err != nil {
		t.Fatalf("Message invalid. %s", err)
	}
	return receivedMessage
}
