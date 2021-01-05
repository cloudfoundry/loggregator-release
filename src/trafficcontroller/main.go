package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/loggregator/trafficcontroller/app"
)

func main() {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	conf, err := app.LoadConfig()
	if err != nil {
		log.Panicf("Unable to load config: %s", err)
	}
	if conf.UseRFC339 {
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	credentials, err := plumbing.NewClientCredentials(
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
		conf.Agent.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.trafficcontroller"),
		metricemitter.WithPulseInterval(conf.MetricEmitterInterval),
		metricemitter.WithSourceID("traffic_controller"),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}

	tc := app.NewTrafficController(
		conf,
		conf.DisableAccessControl,
		metricClient,
		uaaHTTPClient(conf),
		ccHTTPClient(conf),
	)

	tc.Start()
}

func uaaHTTPClient(conf *app.Config) *http.Client {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(conf.UaaCACert),
	)

	if err != nil {
		log.Fatalf("Unable to create UAA HTTP Client: %s", err)
	}

	tlsConfig.InsecureSkipVerify = conf.SkipCertVerify

	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}
	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}

func ccHTTPClient(conf *app.Config) *http.Client {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(conf.CCTLSClientConfig.CertFile, conf.CCTLSClientConfig.KeyFile),
	).Client(
		tlsconfig.WithAuthorityFromFile(conf.CCTLSClientConfig.CAFile),
		tlsconfig.WithServerName(conf.CCTLSClientConfig.ServerName),
	)

	if err != nil {
		log.Fatalf("Unable to create CC HTTP Client: %s", err)
	}
	tlsConfig.InsecureSkipVerify = conf.SkipCertVerify
	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}
	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}
