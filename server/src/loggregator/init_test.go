package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"os"
)

func logger() *gosteno.Logger {
	if os.Getenv("LOG_TO_STDOUT") == "true" {
		level := gosteno.LOG_DEBUG

		loggingConfig := &gosteno.Config{
			Sinks:     make([]gosteno.Sink, 1),
			Level:     level,
			Codec:     gosteno.NewJsonCodec(),
			EnableLOC: true,
		}

		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)

		gosteno.Init(loggingConfig)
	}

	return gosteno.NewLogger("TestLogger")
}
