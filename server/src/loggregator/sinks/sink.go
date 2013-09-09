package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type Sink interface {
	instrumentation.Instrumentable
	Run(sinkCloseChan chan chan *logmessage.Message)
	ListenerChannel() chan *logmessage.Message
	Identifier() string
}
