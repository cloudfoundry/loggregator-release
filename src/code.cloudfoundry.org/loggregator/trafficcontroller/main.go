package main

import (
	"code.cloudfoundry.org/loggregator/plumbing"
	"flag"
	"fmt"
	"log"
	"metricemitter"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/trafficcontroller/app"
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
	metricClient, err := metricemitter.NewClient(
		conf.MetronConfig.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.trafficcontroller"),
		metricemitter.WithDeployment(conf.DeploymentName, conf.JobName, conf.Index),
		metricemitter.WithPulseInterval(conf.MetricEmitterDuration),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}
	tc := app.NewTrafficController(conf, *logFilePath, *disableAccessControl, metricClient)
	tc.Start()
}
