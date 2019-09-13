package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/go-loggregator/metrics"
	"code.cloudfoundry.org/loggregator/profiler"
	"code.cloudfoundry.org/loggregator/rlp-gateway/app"
)

func main() {
	loggr := log.New(os.Stderr, "", log.LstdFlags)

	loggr.Println("starting RLP gateway")
	defer loggr.Println("stopping RLP gateway")

	cfg := app.LoadConfig()
	m := metrics.NewRegistry(
		loggr,
		metrics.WithTLSServer(
			int(cfg.MetricsServer.Port),
			cfg.MetricsServer.CertFile,
			cfg.MetricsServer.KeyFile,
			cfg.MetricsServer.CAFile,
		),
	)

	go profiler.New(cfg.PProfPort).Start()
	app.NewGateway(cfg, m, loggr, os.Stdout).Start(true)
}
