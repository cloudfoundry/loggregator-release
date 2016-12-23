package main

import (
	"flag"
	"log"

	"statsd-injector/statsdemitter"
	"statsd-injector/statsdlistener"

	"github.com/cloudfoundry/sonde-go/events"
)

var (
	statsdPort = flag.Uint("statsdPort", 8125, "The UDP port the injector will listen on for statsd messages")
	metronPort = flag.Uint("metronPort", 51161, "The UDP port the injector will forward message to")
)

func main() {
	flag.Parse()
	log.Print("Starting statsd injector")
	defer log.Print("statsd injector closing")

	statsdMessageListener := statsdlistener.New(*statsdPort)
	statsdEmitter := statsdemitter.New(*metronPort)

	inputChan := make(chan *events.Envelope)

	go statsdMessageListener.Run(inputChan)
	statsdEmitter.Run(inputChan)
}
