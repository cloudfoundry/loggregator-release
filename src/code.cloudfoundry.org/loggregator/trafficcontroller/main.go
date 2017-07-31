package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/loggregator/trafficcontroller/app"
)

func main() {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	disableAccessControl := flag.Bool(
		"disableAccessControl",
		false,
		"always all access to app logs",
	)
	configFile := flag.String(
		"config",
		"config/loggregator_trafficcontroller.json",
		"Location of the loggregator trafficcontroller config json file",
	)

	flag.Parse()

	conf, err := app.ParseConfig(*configFile)
	if err != nil {
		log.Panicf("Unable to parse config: %s", err)
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
		conf.MetronConfig.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.trafficcontroller"),
		metricemitter.WithPulseInterval(conf.MetricEmitterDuration),
	)
	if err != nil {
		log.Fatalf("Couldn't connect to metric emitter: %s", err)
	}

	tc := app.NewTrafficController(
		conf,
		*disableAccessControl,
		metricClient,
		uaaHTTPClient(conf),
		ccHTTPClient(conf),
	)
	tc.Start()
}

func uaaHTTPClient(conf *app.Config) *http.Client {
	tlsConfig := plumbing.NewTLSConfig()
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
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		conf.CCTLSClientConfig.CertFile,
		conf.CCTLSClientConfig.KeyFile,
		conf.CCTLSClientConfig.CAFile,
		conf.CCTLSClientConfig.ServerName,
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
