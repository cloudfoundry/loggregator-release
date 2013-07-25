package instrumentor

import (
	"github.com/cloudfoundry/gosteno"
	"time"
)

type instrumentor struct {
	interval        time.Duration
	minimumLogLevel gosteno.LogLevel
	logger          gosteno.L
}

type PropVal struct {
	Property string
	Value    string
}

type Instrumentable interface {
	DumpData() []PropVal
}

func NewInstrumentor(interval time.Duration, minimumLogLevel gosteno.LogLevel, logger gosteno.L) *instrumentor {
	return &instrumentor{interval, minimumLogLevel, logger}
}

func (instrumentor *instrumentor) Instrument(instrumented Instrumentable) chan bool {
	stopChannel := make(chan bool)
	if instrumentor.minimumLogLevel.Priority >= instrumentor.logger.Level().Priority {
		go func() {
			for {
				select {
				case <-stopChannel:
					return
				case <-time.After(instrumentor.interval):
					//continue
				}

				for _, propval := range instrumented.DumpData() {
					instrumentor.logger.Log(instrumentor.minimumLogLevel, propval.Property+": "+propval.Value, nil)
				}
			}
		}()
	}
	return stopChannel
}

func (instrumentor *instrumentor) StopInstrumentation(stopChannel chan bool) {
	stopChannel <- true
}
