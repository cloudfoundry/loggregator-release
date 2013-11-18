package buffer

import "github.com/cloudfoundry/loggregatorlib/logmessage"

type MessageBuffer interface {
	SetOutputChannel(out chan *logmessage.Message)
	GetOutputChannel() <-chan *logmessage.Message
	CloseOutputChannel()
	Run()
}
