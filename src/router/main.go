package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator/profiler"
	"code.cloudfoundry.org/loggregator/router/app"
	"google.golang.org/grpc/grpclog"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	rand.Seed(time.Now().UnixNano())
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	conf, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}

	envstruct.WriteReport(conf)

	r := app.NewRouter(
		conf.GRPC,
		app.WithPersistence(
			conf.MaxRetainedLogMessages,
			conf.SinkInactivityTimeoutSeconds,
		),
		app.WithMetricReporting(
			conf.HealthAddr,
			conf.Agent,
			conf.MetricBatchIntervalMilliseconds,
			conf.MetricSourceID,
		),
	)
	r.Start()

	profiler.New(conf.PProfPort).Start()
}
