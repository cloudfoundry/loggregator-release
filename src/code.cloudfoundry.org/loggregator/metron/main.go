package main

import (
	"flag"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/loggregator/profiler"

	"code.cloudfoundry.org/loggregator/metron/app"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	configFilePath := flag.String(
		"config",
		"config/metron.json",
		"Location of the Metron config json file",
	)
	flag.Parse()

	config, err := app.ParseConfig(*configFilePath)
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}

	metron := app.NewMetron(config)
	go metron.Start()

	// We start the profiler last so that we can definitively say that we're
	// all connected and ready for data by the time the profiler starts up.
	profiler.New(config.PPROFPort).Start()
}
