package main

import (
	"flag"
	"fmt"
	"profiler"

	"metron/api"
	"metron/config"
)

func main() {
	configFilePath := flag.String(
		"config",
		"config/metron.json",
		"Location of the Metron config json file",
	)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	appV1 := &api.AppV1{}
	go appV1.Start(config)

	appV2 := &api.AppV2{}
	go appV2.Start(config)

	// We start the profiler last so that we can definitively say that we're
	// all connected and ready for data by the time the profiler starts up.
	profiler.New(config.PPROFPort).Start()
}
