package deaagent_testhelpers

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"os"
	"github.com/cloudfoundry/gosteno"
	"testing"
)

func GetBackendMessage(t *testing.T, data *[]byte) *logmessage.LogMessage {
	receivedMessage := &logmessage.LogMessage{}

	err := proto.Unmarshal(*data, receivedMessage)

	if err != nil {
		t.Fatalf("Message invalid. %s", err)
	}
	return receivedMessage
}


func Logger() *gosteno.Logger {
	return getLogger(false)
}

func getLogger(debug bool) *gosteno.Logger {
	if debug {
		level := gosteno.LOG_DEBUG

		loggingConfig := &gosteno.Config{
			Sinks:     make([]gosteno.Sink, 1),
			Level:     level,
			Codec:     gosteno.NewJsonCodec(),
			EnableLOC: true,
		}

		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)

		gosteno.Init(loggingConfig)
	}

	return gosteno.NewLogger("TestLogger")
}
