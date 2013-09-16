package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"runtime"
	testhelpers "server_testhelpers"
)

type testSink struct {
	name string
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
	return testhelpers.Logger()
}

func (sink *testSink) Run(sinkCloseChan chan Sink) {
	alreadyRequestedClose := false
	for {
		runtime.Gosched()
		requestClose(sink, sinkCloseChan, &alreadyRequestedClose)
	}
}

func (sink *testSink) Emit() instrumentation.Context {
	return instrumentation.Context{}

}
