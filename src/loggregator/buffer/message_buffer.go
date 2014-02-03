package buffer

import "github.com/cloudfoundry/loggregatorlib/logmessage"

type MessageBuffer interface {
	GetOutputChannel() <-chan *logmessage.Message
	CloseOutputChannel()
	Run()
}
