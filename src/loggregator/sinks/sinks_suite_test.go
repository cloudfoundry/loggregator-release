package sinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinks"
	"runtime"
	"testing"
	"time"
)

func NewMessage(messageString, appId string) *logmessage.Message {
	logMessage := generateLogMessage(messageString, appId, logmessage.LogMessage_OUT, "App", "")

	marshalledLogMessage, _ := proto.Marshal(logMessage)

	return logmessage.NewMessage(logMessage, marshalledLogMessage)
}

func NewLogMessage(messageString, appId string) *logmessage.LogMessage {
	messageType := logmessage.LogMessage_OUT
	sourceName := "App"

	return generateLogMessage(messageString, appId, messageType, sourceName, "")
}

func generateLogMessage(messageString, appId string, messageType logmessage.LogMessage_MessageType, sourceName, sourceId string) *logmessage.LogMessage {
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: &messageType,
		SourceName:  proto.String(sourceName),
		SourceId:    proto.String(sourceId),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}

type testSink struct {
	name          string
	sinkCloseChan chan sinks.Sink
}

func (sink *testSink) AppId() string {
	return ""
}

func (sink *testSink) Channel() chan *logmessage.Message {
	return make(chan *logmessage.Message)
}

func (sink *testSink) Identifier() string {
	return ""
}

func (sink *testSink) Logger() *gosteno.Logger {
	return loggertesthelper.Logger()
}

func (sink *testSink) Run() {
	alreadyRequestedClose := false
	for {
		runtime.Gosched()
		sinks.RequestClose(sink, sink.sinkCloseChan, &alreadyRequestedClose)
	}
}

func (sink *testSink) ShouldReceiveErrors() bool {
	return false
}

func (sink *testSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

func TestSinks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinks Suite")
}
