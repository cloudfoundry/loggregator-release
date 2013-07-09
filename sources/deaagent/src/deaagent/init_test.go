package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"logMessage"
	"testing"
)

func logger() *gosteno.Logger {
	return gosteno.NewLogger("TestLogger")
}

func getBackendMessage(t *testing.T, data *[]byte) *logMessage.LogMessage {
	receivedMessage := &logMessage.LogMessage{}

	err := proto.Unmarshal(*data, receivedMessage)

	if err != nil {
		t.Fatalf("Message invalid. %s", err)
	}
	return receivedMessage
}
