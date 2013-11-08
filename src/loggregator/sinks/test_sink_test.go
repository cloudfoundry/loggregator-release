package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"runtime"
)

type testSink struct {
	name          string
	sinkCloseChan chan Sink
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
		requestClose(sink, sink.sinkCloseChan, &alreadyRequestedClose)
	}
}

func (sink *testSink) Emit() instrumentation.Context {
	return instrumentation.Context{}

}
