package buffer

import (
	"doppler/envelopewrapper"
)

type MessageBuffer interface {
	GetOutputChannel() <-chan *envelopewrapper.WrappedEnvelope
	CloseOutputChannel()
	Run()
}
