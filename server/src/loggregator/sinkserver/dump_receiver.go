package sinkserver

import "github.com/cloudfoundry/loggregatorlib/logmessage"

type dumpReceiver struct {
	outputChannel chan logmessage.Message
	appId         string
}
