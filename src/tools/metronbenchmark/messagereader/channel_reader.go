package messagereader

import "github.com/cloudfoundry/sonde-go/events"

type channelReader struct {
	c <-chan *events.LogMessage
}

func NewChannelReader(c <-chan *events.LogMessage) *channelReader {
	return &channelReader{c: c}
}

func (cr *channelReader) Read() (*events.LogMessage, bool) {
	lm, ok := <-cr.c
	return lm, ok
}
