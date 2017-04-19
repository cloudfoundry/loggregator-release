package main

import (
	"flag"
	"fmt"
	"log"
	"metric"
	"plumbing"

	"google.golang.org/grpc"

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

	credentials, err := plumbing.NewCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for client: %s", err)
	}

	// metric-documentation-v2: setup function
	metric.Setup(
		metric.WithGrpcDialOpts(grpc.WithTransportCredentials(credentials)),
		metric.WithOrigin("loggregator.trafficcontroller"),
		metric.WithAddr(conf.MetronConfig.GRPCAddress),
		metric.WithDeploymentMeta(conf.DeploymentName, conf.JobName, conf.Index),
	)
	tc := app.NewTrafficController(conf, *logFilePath, *disableAccessControl)
	tc.Start()
}
