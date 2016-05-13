package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metrics"
)

var (
	metronHostPort string
	filter         string
	origin         string
	delay          time.Duration
)

func init() {
	flag.StringVar(&metronHostPort, "target", "127.0.0.1:3457", "host:port for the target metron agent")
	flag.StringVar(&filter, "filter", "", "send envelopes of a specific type such as LogMessage, ValueMetric, CounterEvent, HttpStartStop")
	flag.StringVar(&origin, "origin", "envelope-emitter", "origin field of the envelope")
	flag.DurationVar(&delay, "delay", time.Second, "delay between sending metrics")
}

func main() {
	flag.Parse()
	dropsonde.Initialize(metronHostPort, origin)

	for {
		fmt.Printf(".")

		switch filter {
		// TODO: Add support for other event types as we add chaining APIs for
		// those event types in NOAA.

		//case "LogMessage":
		//case "HttpStartStop":
		//case "HttpStart":
		//case "HttpStop":
		//case "Error":

		case "CounterEvent":
			sendCounterEvent()
		case "ValueMetric":
			sendValueMetric()
		case "ContainerMetric":
			sendContainerMetric()
		default:
			sendCounterEvent()
			sendValueMetric()
			sendContainerMetric()
		}

		time.Sleep(delay)
	}
}

var counter metric_sender.CounterChainer

func sendCounterEvent() {
	if counter == nil {
		counter = metrics.Counter("requests").SetTag("protocol", "http")
	}
	err := counter.Increment()
	if err != nil {
		panic(err)
	}
	err = counter.Add(3)
	if err != nil {
		panic(err)
	}
}

func sendValueMetric() {
	err := metrics.Value("current-air-pressure", 101.325, "kNm^-2").
		SetTag("example-tag", "foo").
		Send()
	if err != nil {
		panic(err)
	}
}

func sendContainerMetric() {
	err := metrics.ContainerMetric("fake-app-id", 1, 58.2, 13, 41).
		SetTag("example-tag", "foo").
		Send()
	if err != nil {
		panic(err)
	}
}
