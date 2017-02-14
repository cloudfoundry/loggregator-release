package main

import (
	"flag"
	"fmt"
	"log"

	"statsd-injector/statsdemitter"
	"statsd-injector/statsdlistener"

	"github.com/cloudfoundry/sonde-go/events"
)

var (
	statsdHost = flag.String("statsdHost", "localhost", "The hostname the injector will listen on for statsd messages")
	statsdPort = flag.Uint("statsdPort", 8125, "The UDP port the injector will listen on for statsd messages")
	metronPort = flag.Uint("metronPort", 51161, "The UDP port the injector will forward message to")
)

func main() {
	flag.Parse()

	log.Print("Starting statsd injector")
	defer log.Print("statsd injector closing")

	hostport := fmt.Sprintf("%s:%d", *statsdHost, *statsdPort)
	statsdMessageListener := statsdlistener.New(hostport)
	statsdEmitter := statsdemitter.New(*metronPort)

	inputChan := make(chan *events.Envelope)

	go statsdMessageListener.Run(inputChan)
	statsdEmitter.Run(inputChan)
}
