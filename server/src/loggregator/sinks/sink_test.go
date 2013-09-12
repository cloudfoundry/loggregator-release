package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"runtime"
	testhelpers "server_testhelpers"
	"testing"
	"time"
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

func TestThatOnlyOneRequestCloseOccurs(t *testing.T) {
	closeChan := make(chan Sink)

	sink := testSink{"1"}
	go sink.Run(closeChan)
	runtime.Gosched()

	closeSink := <-closeChan
	assert.Equal(t, &sink, closeSink)

	select {
	case <-closeChan:
		t.Error("Should not have received value on closeChan")
	case <-time.After(50 * time.Millisecond):
	}

}
