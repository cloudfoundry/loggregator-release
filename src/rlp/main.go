package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator/metricemitter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/profiler"

	"code.cloudfoundry.org/loggregator/rlp/app"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	conf, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}

	envstruct.WriteReport(conf)

	dopplerCredentials, err := plumbing.NewClientCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	var opts []plumbing.ConfigOption
	if len(conf.GRPC.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(conf.GRPC.CipherSuites))
	}
	rlpCredentials, err := plumbing.NewServerCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		opts...,
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	metronCredentials, err := plumbing.NewClientCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use TLS config: %s", err)
	}

	// metric-documentation-v2: setup function
	metric, err := metricemitter.NewClient(
		conf.AgentAddr,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(metronCredentials)),
		metricemitter.WithOrigin("loggregator.rlp"),
		metricemitter.WithPulseInterval(conf.MetricEmitterInterval),
		metricemitter.WithSourceID(conf.MetricSourceID),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}

	ingressKP := keepalive.ClientParameters{
		Time:                15 * time.Second,
		Timeout:             20 * time.Second,
		PermitWithoutStream: true,
	}
	egressKP := keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}
	rlp := app.NewRLP(
		metric,
		app.WithEgressPort(conf.GRPC.Port),
		app.WithIngressAddrs(conf.RouterAddrs),
		app.WithIngressDialOptions(
			grpc.WithTransportCredentials(dopplerCredentials),
			grpc.WithKeepaliveParams(ingressKP),
		),
		app.WithEgressServerOptions(
			grpc.Creds(rlpCredentials),
			grpc.KeepaliveEnforcementPolicy(egressKP),
		),
		app.WithHealthAddr(conf.HealthAddr),
		app.WithMaxEgressStreams(conf.MaxEgressStreams),
	)
	go rlp.Start()
	defer rlp.Stop()
	go profiler.New(conf.PProfPort).Start()

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM)
	<-killSignal
}
