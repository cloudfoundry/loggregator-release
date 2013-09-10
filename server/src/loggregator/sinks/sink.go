package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type Sink interface {
	instrumentation.Instrumentable
	AppId() string
	Run(sinkCloseChan chan Sink)
	ListenerChannel() chan *logmessage.Message
	Identifier() string
}
