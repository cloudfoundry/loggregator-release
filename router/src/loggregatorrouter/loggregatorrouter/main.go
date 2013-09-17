package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"loggregatorrouter"
	"net"
	"os"
	"os/signal"
	"strconv"
)

type Config struct {
	cfcomponent.Config
	Host         string
	Loggregators []string
	SystemDomain string
	WebPort      uint32
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}
	if len(c.Loggregators) < 1 || c.Loggregators[0] == "" {
		return errors.New("Need a loggregator server (host:port).")
	}

	err = c.Validate(logger)
	return
}

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	version     = flag.Bool("version", false, "Version info")
	configFile  = flag.String("config", "config/loggregator_router.json", "Location of the loggregator router config json file")
)

const (
	versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
	gitSha        = `TRAVIS_COMMIT`
)

func main() {
	flag.Parse()

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "udprouter")

	if *version {
		fmt.Printf("version: %s\ngitSha: %s\nsourceUrl: https://github.com/cloudfoundry/loggregator/tree/%s\n",
			versionNumber, gitSha, gitSha)
		return
	}
	config := &Config{Host: "0.0.0.0:3456", WebPort: 8080}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	h := loggregatorrouter.NewHasher(config.Loggregators)
	r, err := loggregatorrouter.NewRouter(config.Host, h, config.Config, logger)
	if err != nil {
		panic(err)
	}

	redirector := loggregatorrouter.NewRedirector(net.JoinHostPort(r.Component.IpAddress, strconv.Itoa(int(config.WebPort))), h, logger)
	go func() {
		err := redirector.Start()
		if err != nil {
			panic(err)
		}
	}()

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(r.Component.IpAddress, config.WebPort, []string{uri})
	if err != nil {
		logger.Fatalf("Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(r.Component)
	if err != nil {
		panic(err)
	}

	go func() {
		err := r.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()

	go r.Start(logger)

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			rr.UnregisterFromRouter(r.Component.IpAddress, config.WebPort, []string{uri})
			break
		}
	}
}
