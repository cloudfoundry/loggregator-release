package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"net"
	"os"
	"os/signal"
	"strconv"
	"trafficcontroller"
	"trafficcontroller/hasher"
)

type Config struct {
	Zone string
	cfcomponent.Config
	Host         string
	Loggregators map[string][]string
	IncomingPort uint32
	OutgoingPort uint32
	SystemDomain string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}
	if len(c.Loggregators) < 1 {
		return errors.New("Need a loggregator server (host:port).")
	}

	err = c.Validate(logger)
	return
}

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	version     = flag.Bool("version", false, "Version info")
	configFile  = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
)

const (
	versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
	gitSha        = `TRAVIS_COMMIT`
)

func main() {
	flag.Parse()

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "udprouter")
	logger.Debugf("Setting up the loggregator traffic controller")

	if *version {
		fmt.Printf("version: %s\ngitSha: %s\nsourceUrl: https://github.com/cloudfoundry/loggregator/tree/%s\n\n",
			versionNumber, gitSha, gitSha)
		return
	}
	config := &Config{OutgoingPort: 8080}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	config.Host = net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.IncomingPort), 10))

	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	servers := make([]string, len(config.Loggregators[config.Zone]))
	copy(servers, config.Loggregators[config.Zone])
	for index, server := range servers {
		logger.Debugf("Got loggregator server at this address: %v", net.JoinHostPort(server, strconv.FormatUint(uint64(config.IncomingPort), 10)))
		servers[index] = net.JoinHostPort(server, strconv.FormatUint(uint64(config.IncomingPort), 10))
	}
	h := hasher.NewHasher(servers)
	logger.Debugf("Loggregator Server in the zone: %v", config.Loggregators[config.Zone])
	logger.Debugf("Hashed Loggregator Server in the zone: %v", h.LoggregatorServers())
	logger.Debugf("Going to start incoming router on %v", config.Host)
	r, err := trafficcontroller.NewRouter(config.Host, h, config.Config, logger)
	if err != nil {
		panic(err)
	}

	hashers := make([]*hasher.Hasher, len(config.Loggregators))
	logger.Debugf("Number of zones: %v", len(config.Loggregators))
	counter := 0
	for _, servers := range config.Loggregators {
		logger.Debugf("Hashing server: %v", servers)
		for index, server := range servers {
			servers[index] = net.JoinHostPort(server, strconv.FormatUint(uint64(config.OutgoingPort), 10))
		}
		hashers[counter] = hasher.NewHasher(servers)
		counter++
	}
	logger.Debugf("Number of hashers for the proxy: %v", len(hashers))
	proxy := trafficcontroller.NewProxy(net.JoinHostPort(r.Component.IpAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), hashers, logger)
	go func() {
		err := proxy.Start()
		if err != nil {
			panic(err)
		}
	}()

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(r.Component.IpAddress, config.OutgoingPort, []string{uri})
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
			rr.UnregisterFromRouter(r.Component.IpAddress, config.OutgoingPort, []string{uri})
			break
		}
	}
}
