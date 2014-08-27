package buffer

import "github.com/cloudfoundry/dropsonde/events"

type MessageBuffer interface {
	GetOutputChannel() <-chan *events.Envelope
	CloseOutputChannel()
	Run()
}
