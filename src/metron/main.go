package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"metricemitter"
	"time"

	"google.golang.org/grpc"

	"metron/app"
	"metron/internal/health"
	"plumbing"
	"profiler"
)

func main() {
	rand.Seed(time.Now().UnixNano())

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

	server, registry := health.New(config.HealthEndpointPort)
	go server.Run()

	appV1 := app.NewV1App(config, registry, clientCreds)
	go appV1.Start()

	appV2 := app.NewV2App(config, clientCreds, serverCreds, metricClient)
	go appV2.Start()

	// We start the profiler last so that we can definitively say that we're
	// all connected and ready for data by the time the profiler starts up.
	profiler.New(config.PPROFPort).Start()
}
