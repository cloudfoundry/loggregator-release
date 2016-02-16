package main

import (
	"flag"
	"os"

	"statsd-injector/statsdemitter"
	"statsd-injector/statsdlistener"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

var (
	statsdPort = flag.Uint("statsdPort", 8125, "The UDP port the injector will listen on for statsd messages")
	metronPort = flag.Uint("metronPort", 51161, "The UDP port the injector will forward message to")
	logLevel   = flag.String("logLevel", "info", "The logging level")
)

func main() {
	flag.Parse()

	level, err := gosteno.GetLogLevel(*logLevel)
	if err != nil {
		level = gosteno.LOG_INFO
	}
	loggingConfig := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewIOSink(os.Stdout),
		},
		Level:     level,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true,
	}

	gosteno.Init(loggingConfig)
	logger := gosteno.NewLogger("statsdinjector")

	statsdMessageListener := statsdlistener.New(*statsdPort, logger)
	statsdEmitter := statsdemitter.New(*metronPort, logger)

	inputChan := make(chan *events.Envelope)

	go statsdMessageListener.Run(inputChan)
	statsdEmitter.Run(inputChan)
}
