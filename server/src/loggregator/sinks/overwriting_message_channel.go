package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

func OverwritingMessageChannel(in <-chan *logmessage.Message, out chan *logmessage.Message, logger *gosteno.Logger) {
	for v := range in {
		select {
		case out <- v:
		default:
			<-out
			out <- v
			if logger != nil {
				logger.Warnf("OMC: Reader was too slow. Dropped message.")
			}
		}
	}
	close(out)
}
