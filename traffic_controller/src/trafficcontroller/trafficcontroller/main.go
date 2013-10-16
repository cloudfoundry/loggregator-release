package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"io/ioutil"
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
	ApiHost                         string
	Host                            string
	Loggregators                    map[string][]string
	IncomingPort                    uint32
	OutgoingPort                    uint32
	SystemDomain                    string
	decoder                         authorization.TokenDecoder
	DisableEmailDomainAuthorization bool
	UaaVerificationKeyFile          string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}
	if len(c.Loggregators) < 1 {
		return errors.New("Need a loggregator server (host:port).")
	}

	uaaVerificationKey, err := ioutil.ReadFile(c.UaaVerificationKeyFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not read UAA verification key from file %s: %s", c.UaaVerificationKeyFile, err))
	}

	c.decoder, err = authorization.NewUaaTokenDecoder(uaaVerificationKey)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not parse UAA verification key: %s", err))
	}

	err = c.Validate(logger)
	return
}

var (
	logFilePath            = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel               = flag.Bool("debug", false, "Debug logging")
	version                = flag.Bool("version", false, "Version info")
	configFile             = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
	uaaVerificationKeyFile = flag.String("tokenFile", "config/uaa_token.pub", "Location of the loggregator's uaa public token file")
)

const (
	versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
	gitSha        = `TRAVIS_COMMIT`
)

func main() {
	flag.Parse()

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller")
	logger.Debugf("Startup: Setting up the loggregator traffic controller")

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

	makeIncomingRouter := func() *trafficcontroller.Router {
		servers := make([]string, len(config.Loggregators[config.Zone]))
		copy(servers, config.Loggregators[config.Zone])
		for index, server := range servers {
			logger.Debugf("Incoming Router Startup: Forwarding messages from source to loggregator server [%v] at %v", index, net.JoinHostPort(server, strconv.FormatUint(uint64(config.IncomingPort), 10)))
			servers[index] = net.JoinHostPort(server, strconv.FormatUint(uint64(config.IncomingPort), 10))
		}
		h := hasher.NewHasher(servers)
		logger.Debugf("Incoming Router Startup: Loggregator Servers in the zone: %v", config.Loggregators[config.Zone])
		logger.Debugf("Incoming Router Startup: Hashed Loggregator Server in the zone: %v", h.LoggregatorServers())
		logger.Debugf("Incoming Router Startup: Going to start incoming router on %v", config.Host)
		router, err := trafficcontroller.NewRouter(config.Host, h, config.Config, logger)
		if err != nil {
			panic(err)
		}
		return router
	}

	startIncomingRouter := func(router *trafficcontroller.Router) {
		go router.Start(logger)
	}

	setupMonitoring := func(router *trafficcontroller.Router) {
		cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
		err = cr.RegisterWithCollector(router.Component)
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

	authorizer := authorization.NewLogAccessAuthorizer(config.decoder, config.ApiHost, config.DisableEmailDomainAuthorization)

	makeOutgoingProxy := func(ipAddress string) *trafficcontroller.Proxy {
		hashers := make([]*hasher.Hasher, len(config.Loggregators))
		logger.Debugf("Output Proxy Startup: Number of zones: %v", len(config.Loggregators))
		counter := 0
		for _, servers := range config.Loggregators {
			logger.Debugf("Output Proxy Startup: Hashing servers: %v", servers)
			for index, server := range servers {
				logger.Debugf("Output Proxy Startup: Forwarding messages to client from loggregator server [%v] at %v", index, net.JoinHostPort(server, strconv.FormatUint(uint64(config.OutgoingPort), 10)))
				servers[index] = net.JoinHostPort(server, strconv.FormatUint(uint64(config.OutgoingPort), 10))
			}
			hashers[counter] = hasher.NewHasher(servers)
			counter++
		}
		logger.Debugf("Output Proxy Startup: Number of hashers for the proxy: %v", len(hashers))
		proxy := trafficcontroller.NewProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), hashers, authorizer, logger)
		return proxy
	}

	startOutgoingProxy := func(proxy *trafficcontroller.Proxy) {
		go func() {
			err := proxy.Start()
			if err != nil {
				panic(err)
			}
		}()
	}

	router := makeIncomingRouter()
	startIncomingRouter(router)

	proxy := makeOutgoingProxy(router.Component.IpAddress)
	startOutgoingProxy(proxy)

	setupMonitoring(router)

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
