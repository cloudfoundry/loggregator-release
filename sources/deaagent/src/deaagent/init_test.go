package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"logMessage"
	//		"os"
	"testing"
)

func logger() *gosteno.Logger {
	//		level := gosteno.LOG_DEBUG
	//
	//		loggingConfig := &gosteno.Config{
	//			Sinks:     make([]gosteno.Sink, 1),
	//			Level:     level,
	//			Codec:     gosteno.NewJsonCodec(),
	//			EnableLOC: true,
	//		}
	//
	//		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	//
	//		gosteno.Init(loggingConfig)
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
