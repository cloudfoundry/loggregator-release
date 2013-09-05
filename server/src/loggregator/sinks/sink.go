package sinks

import "github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"

type Sink interface {
	instrumentation.Instrumentable
	Run(sinkCloseChan chan chan []byte)
	ListenerChannel() chan []byte
}
