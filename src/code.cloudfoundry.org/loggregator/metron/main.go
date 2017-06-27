package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"code.cloudfoundry.org/loggregator/infofile"
	"code.cloudfoundry.org/loggregator/metricemitter"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/profiler"

	"code.cloudfoundry.org/loggregator/metron/app"
	"code.cloudfoundry.org/loggregator/metron/internal/health"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	configFilePath := flag.String(
		"config",
		"config/metron.json",
		"Location of the Metron config json file",
	)

	infoFilePath := flag.String(
		"info-path",
		"",
		"Path to the info file",
	)

	flag.Parse()

	config, err := app.ParseConfig(*configFilePath)
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}

	clientCreds, err := plumbing.NewCredentials(
		config.GRPC.CertFile,
		config.GRPC.KeyFile,
		config.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for client: %s", err)
	}

	serverCreds, err := plumbing.NewCredentials(
		config.GRPC.CertFile,
		config.GRPC.KeyFile,
		config.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	metricsCreds, err := plumbing.NewCredentials(
		config.GRPC.CertFile,
		config.GRPC.KeyFile,
		config.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(config.MetricBatchIntervalMilliseconds) * time.Millisecond
	// metric-documentation-v2: setup function
	metricClient, err := metricemitter.NewClient(
		fmt.Sprintf("localhost:%d", config.GRPC.Port),
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(metricsCreds)),
		metricemitter.WithOrigin("loggregator.metron"),
		metricemitter.WithDeployment(config.Deployment, config.Job, config.Index),
		metricemitter.WithPulseInterval(batchInterval),
	)
	if err != nil {
		log.Fatalf("Could not configure metric emitter: %s", err)
	}

	var opts []infofile.Option
	if *infoFilePath != "" {
		opts = append(opts, infofile.WithPath(*infoFilePath))
	}
	info := infofile.New("metron", opts...)

	server, registry := health.New(config.HealthEndpointPort)
	go server.Run()
	info.Set("health_url", fmt.Sprintf("http://%s/health", server.Addr()))

	appV1 := app.NewV1App(config, registry, clientCreds)
	go appV1.Start()
	info.Set("udp_addr", appV1.Addr().String())

	appV2 := app.NewV2App(config, registry, clientCreds, serverCreds, metricClient)
	go appV2.Start()
	info.Set("grpc_addr", appV2.Addr().String())

	// We start the profiler last so that we can definitively say that we're
	// all connected and ready for data by the time the profiler starts up.
	profiler.New(config.PPROFPort).Start()
}
