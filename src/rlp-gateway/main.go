package main

import (
	"log"
	"os"

	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/loggregator-release/plumbing"
	"code.cloudfoundry.org/loggregator-release/profiler"
	"code.cloudfoundry.org/loggregator-release/rlp-gateway/app"
)

func main() {
	loggr := log.New(os.Stderr, "", log.LstdFlags)
	cfg := app.LoadConfig()
	if cfg.UseRFC339 {
		loggr = log.New(new(plumbing.LogWriter), "", 0)
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	}
	loggr.Println("starting RLP gateway")
	defer loggr.Println("stopping RLP gateway")
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
