package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"metric"
	"profiler"
	"runtime"
	"time"

	"google.golang.org/grpc"

	"metron/app"
	"plumbing"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	configFilePath := flag.String(
		"config",
		"config/metron.json",
		"Location of the Metron config json file",
	)
	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

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

	appV1 := app.NewV1App(config, clientCreds)
	go appV1.Start()

	appV2 := app.NewV2App(config, clientCreds, serverCreds)
	go appV2.Start()

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
	metric.Setup(
		metric.WithGrpcDialOpts(grpc.WithTransportCredentials(metricsCreds)),
		metric.WithBatchInterval(batchInterval),
		metric.WithOrigin("loggregator.metron"),
		metric.WithAddr(fmt.Sprintf("localhost:%d", config.GRPC.Port)),
		metric.WithDeploymentMeta(config.Deployment, config.Job, config.Index),
	)

	// We start the profiler last so that we can definitively say that we're
	// all connected and ready for data by the time the profiler starts up.
	profiler.New(config.PPROFPort).Start()
}
