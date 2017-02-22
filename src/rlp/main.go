package main

import (
	"flag"
	"log"
	"strings"

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

	flag.Parse()

	tlsCredentials, err := plumbing.NewCredentials(
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

	rlp := app.NewRLP(
		app.WithEgressPort(*egressPort),
		app.WithIngressAddrs(hostPorts),
		app.WithIngressDialOptions(grpc.WithTransportCredentials(tlsCredentials)),
		app.WithEgressServerOptions(grpc.Creds(tlsCredentials)),
	)
	go rlp.Start()

	profiler.New(uint32(*pprofPort)).Start()
}
