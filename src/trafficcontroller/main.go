package main

import (
	"flag"
	"fmt"

	"trafficcontroller/app"
)

func main() {
	logFilePath := flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	disableAccessControl := flag.Bool("disableAccessControl", false, "always all access to app logs")
	configFile := flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
	flag.Parse()

	conf, err := app.ParseConfig(*configFile)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	tc := app.NewTrafficController(conf, *logFilePath, *disableAccessControl)
	tc.Start()
}
