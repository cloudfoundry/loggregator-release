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
	"trafficcontroller/authorization"
	"trafficcontroller/hasher"
)

type Config struct {
	Zone string
	cfcomponent.Config
	ApiHost                 string
	Host                    string
	Loggregators            map[string][]string
	LoggregatorIncomingPort uint32
	LoggregatorOutgoingPort uint32
	IncomingPort            uint32
	OutgoingPort            uint32
	SystemDomain            string
	SkipCertVerify          bool
}

func (c *Config) setDefaults() {
	if c.LoggregatorIncomingPort == 0 {
		c.LoggregatorIncomingPort = c.IncomingPort
	}
	if c.LoggregatorOutgoingPort == 0 {
		c.LoggregatorOutgoingPort = c.OutgoingPort
	}
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

	if *version {
		fmt.Printf("version: %s\ngitSha: %s\nsourceUrl: https://github.com/cloudfoundry/loggregator/tree/%s\n\n",
			versionNumber, gitSha, gitSha)
		return
	}

	config, logger, err := parseConfig(logLevel, configFile, logFilePath)
	if err != nil {
		panic(err)
	}

	router := makeIncomingRouter(config, logger)
	startIncomingRouter(router, logger)

	proxy := makeOutgoingProxy(router.Component.IpAddress, config, logger)
	startOutgoingProxy(proxy)

	setupMonitoring(router, config, logger)

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(router.Component.IpAddress, config.OutgoingPort, []string{uri})
	if err != nil {
		logger.Fatalf("Startup: Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			rr.UnregisterFromRouter(router.Component.IpAddress, config.OutgoingPort, []string{uri})
			break
		}
	}
}

func parseConfig(logLevel *bool, configFile, logFilePath *string) (*Config, *gosteno.Logger, error) {
	config := &Config{OutgoingPort: 8080}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		return nil, nil, err
	}

	config.setDefaults()
	config.Host = net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.IncomingPort), 10))
	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller", config.Config)
	logger.Info("Startup: Setting up the loggregator traffic controller")

	err = config.validate(logger)
	if err != nil {
		return nil, nil, err
	}
	return config, logger, nil
}

func makeIncomingRouter(config *Config, logger *gosteno.Logger) *trafficcontroller.Router {
	serversForZone := config.Loggregators[config.Zone]
	servers := make([]string, len(serversForZone))
	for index, server := range serversForZone {
		logger.Debugf("Incoming Router Startup: Forwarding messages from source to loggregator server [%v] at %v", index, net.JoinHostPort(server, strconv.FormatUint(uint64(config.LoggregatorIncomingPort), 10)))
		servers[index] = net.JoinHostPort(serversForZone[index], strconv.FormatUint(uint64(config.LoggregatorIncomingPort), 10))
	}
	logger.Debugf("Incoming Router Startup: Loggregator Servers in the zone %s: %v", config.Zone, servers)

	h := hasher.NewHasher(servers)
	logger.Debugf("Incoming Router Startup: Hashed Loggregator Server in the zone: %v", h.LoggregatorServers())
	logger.Debugf("Incoming Router Startup: Going to start incoming router on %v", config.Host)
	router, err := trafficcontroller.NewRouter(config.Host, h, config.Config, logger)
	if err != nil {
		panic(err)
	}
	return router
}

func makeOutgoingProxy(ipAddress string, config *Config, logger *gosteno.Logger) *trafficcontroller.Proxy {
	authorizer := authorization.NewLogAccessAuthorizer(config.ApiHost, config.SkipCertVerify)

	logger.Debugf("Output Proxy Startup: Number of zones: %v", len(config.Loggregators))
	hashers := makeHashers(config.Loggregators, config.LoggregatorOutgoingPort, logger)

	logger.Debugf("Output Proxy Startup: Number of hashers for the proxy: %v", len(hashers))
	proxy := trafficcontroller.NewProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), hashers, authorizer, logger)
	return proxy
}

func makeHashers(loggregators map[string][]string, loggregatorOutgoingPort uint32, logger *gosteno.Logger) []*hasher.Hasher {
	counter := 0
	hashers := make([]*hasher.Hasher, 0, len(loggregators))
	for _, servers := range loggregators {
		logger.Debugf("Output Proxy Startup: Hashing servers: %v  Length: %d", servers, len(servers))

		if len(servers) == 0 {
			continue
		}

		for index, server := range servers {
			logger.Debugf("Output Proxy Startup: Forwarding messages to client from loggregator server [%v] at %v", index, net.JoinHostPort(server, strconv.FormatUint(uint64(loggregatorOutgoingPort), 10)))
			servers[index] = net.JoinHostPort(server, strconv.FormatUint(uint64(loggregatorOutgoingPort), 10))
		}
		hashers = hashers[:(counter + 1)]
		hashers[counter] = hasher.NewHasher(servers)
		counter++
	}
	return hashers
}

func startIncomingRouter(router *trafficcontroller.Router, logger *gosteno.Logger) {
	go router.Start(logger)
}

func setupMonitoring(router *trafficcontroller.Router, config *Config, logger *gosteno.Logger) {
	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err := cr.RegisterWithCollector(router.Component)
	if err != nil {
		panic(err)
	}

	go func() {
		err := router.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()
}

func startOutgoingProxy(proxy *trafficcontroller.Proxy) {
	go func() {
		err := proxy.Start()
		if err != nil {
			panic(err)
		}
	}()
}
