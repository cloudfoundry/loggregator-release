package main

import (
	"io"
	"log"
	"math/rand"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"code.cloudfoundry.org/loggregator-release/src/profiler"
	"code.cloudfoundry.org/loggregator-release/src/router/app"
	"google.golang.org/grpc/grpclog"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	l := grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard)
	grpclog.SetLoggerV2(l)

	conf, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}
	if conf.UseRFC339 {
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}
	err = envstruct.WriteReport(conf)
	if err != nil {
		log.Printf("Failed to print a report of the from environment: %s\n", err)
	}

	r := app.NewRouter(
		conf.GRPC,
		app.WithBufferSizes(
			conf.IngressBufferSize,
			conf.EgressBufferSize,
		),
		app.WithMetricReporting(
			conf.Agent,
			conf.MetricBatchIntervalMilliseconds,
			conf.MetricSourceID,
		),
	)
	r.Start()

	profiler.New(conf.PProfPort).Start()
}
