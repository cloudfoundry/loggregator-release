package main

import (
	"expvar"
	"log"
	"os"

	"code.cloudfoundry.org/loggregator/profiler"
	"code.cloudfoundry.org/loggregator/rlp-gateway/app"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/metrics"
)

func main() {
	log := log.New(os.Stderr, "", log.LstdFlags)
	metrics := metrics.New(expvar.NewMap("RLPGateway"))

	log.Println("starting RLP gateway")
	defer log.Println("stopping RLP gateway")

	cfg := app.LoadConfig()

	go profiler.New(cfg.PProfPort).Start()
	app.NewGateway(cfg, metrics, log, os.Stdout).Start(true)
}
