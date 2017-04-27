package main

import (
	"flag"
	"log"
	"metricemitter"
	"strings"
	"time"

	"google.golang.org/grpc"

	"plumbing"
	"profiler"
	"rlp/app"
)

func main() {
	egressPort := flag.Int("egress-port", 0, "The port of the Egress server")
	ingressAddrsList := flag.String("ingress-addrs", "", "The addresses of Dopplers")
	pprofPort := flag.Int("pprof-port", 6061, "The port of pprof for health checks")

	caFile := flag.String("ca", "", "The file path for the CA cert")
	certFile := flag.String("cert", "", "The file path for the client cert")
	keyFile := flag.String("key", "", "The file path for the client key")

	metronAddr := flag.String("metron-addr", "localhost:3458", "The GRPC address to inject metrics to")
	batchInterval := flag.Duration("batch-interval", time.Minute, "The interval to send batched metrics to metron")
	job := flag.String("job", "", "The name of the job")
	deployment := flag.String("deployment", "", "The name of the deployment")
	index := flag.String("index", "", "The name of the index")

	flag.Parse()

	dopplerCredentials, err := plumbing.NewCredentials(
		*certFile,
		*keyFile,
		*caFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	hostPorts := strings.Split(*ingressAddrsList, ",")
	if len(hostPorts) == 0 {
		log.Fatal("no Ingress Addrs were provided")
	}

	metronCredentials, err := plumbing.NewCredentials(
		*certFile,
		*keyFile,
		*caFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	metric, err := metricemitter.NewClient(
		*metronAddr,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(metronCredentials)),
		metricemitter.WithOrigin("loggregator.rlp"),
		metricemitter.WithDeployment(*deployment, *job, *index),
		metricemitter.WithPulseInterval(*batchInterval),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}

	rlp := app.NewRLP(
		metric,
		app.WithEgressPort(*egressPort),
		app.WithIngressAddrs(hostPorts),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(dopplerCredentials)),
		app.WithEgressServerOptions(grpc.Creds(dopplerCredentials)),
	)
	go rlp.Start()

	profiler.New(uint32(*pprofPort)).Start()
}
