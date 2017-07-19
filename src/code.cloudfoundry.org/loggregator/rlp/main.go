package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/profiler"

	"code.cloudfoundry.org/loggregator/rlp/app"
)

func main() {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	egressPort := flag.Int("egress-port", 0, "The port of the Egress server")
	ingressAddrsList := flag.String("ingress-addrs", "", "The addresses of Dopplers")
	pprofPort := flag.Int("pprof-port", 6061, "The port of pprof for health checks")
	healthAddr := flag.String("health-addr", "localhost:14825", "The address for the health endpoint")

	caFile := flag.String("ca", "", "The file path for the CA cert")
	certFile := flag.String("cert", "", "The file path for the client cert")
	keyFile := flag.String("key", "", "The file path for the client key")
	rawCipherSuites := flag.String("cipher-suites", "", "The approved cipher suites for TLS. Multiple cipher suites should be separated by a ':'")

	metronAddr := flag.String("metron-addr", "localhost:3458", "The GRPC address to inject metrics to")
	metricEmitterInterval := flag.Duration("metric-emitter-interval", time.Minute, "The interval to send batched metrics to metron")

	flag.Parse()

	dopplerCredentials, err := plumbing.NewClientCredentials(
		*certFile,
		*keyFile,
		*caFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	cipherSuites := strings.Split(*rawCipherSuites, ":")

	var opts []plumbing.ConfigOption
	if len(cipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(cipherSuites))
	}
	rlpCredentials, err := plumbing.NewServerCredentials(
		*certFile,
		*keyFile,
		*caFile,
		opts...,
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	hostPorts := strings.Split(*ingressAddrsList, ",")
	if len(hostPorts) == 0 {
		log.Fatal("no Ingress Addrs were provided")
	}

	metronCredentials, err := plumbing.NewClientCredentials(
		*certFile,
		*keyFile,
		*caFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	// metric-documentation-v2: setup function
	metric, err := metricemitter.NewClient(
		*metronAddr,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(metronCredentials)),
		metricemitter.WithOrigin("loggregator.rlp"),
		metricemitter.WithPulseInterval(*metricEmitterInterval),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}

	rlp := app.NewRLP(
		metric,
		app.WithEgressPort(*egressPort),
		app.WithIngressAddrs(hostPorts),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(dopplerCredentials)),
		app.WithEgressServerOptions(grpc.Creds(rlpCredentials)),
		app.WithHealthAddr(*healthAddr),
	)
	go rlp.Start()
	go profiler.New(uint32(*pprofPort)).Start()
	defer rlp.Stop()

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM)
	<-killSignal
}
