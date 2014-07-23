package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/hasher"
	"trafficcontroller/inputrouter"
	"trafficcontroller/outputproxy"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
)

var DefaultStoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workerPool := workerpool.NewWorkerPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workerPool)
}

var EtcdQueryInterval = 5 * time.Second

type Config struct {
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int

	JobName  string
	JobIndex int
	Zone     string
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

	if c.JobName == "" {
		c.JobName = "loggregator_trafficcontroller"
	}

	if c.EtcdMaxConcurrentRequests == 0 {
		c.EtcdMaxConcurrentRequests = 10
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
	configFile  = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile  = flag.String("memprofile", "", "write memory profile to this file")
)

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(err)
		}
		go func() {
			defer f.Close()
			ticker := time.NewTicker(time.Second * 1)
			defer ticker.Stop()
			for {
				<-ticker.C
				pprof.WriteHeapProfile(f)
			}
		}()
	}

	config, logger, err := ParseConfig(logLevel, configFile, logFilePath)
	if err != nil {
		panic(err)
	}

	router := makeIncomingRouter(config, logger)
	startIncomingRouter(router, logger)

	proxy := makeOutgoingProxy(config, logger)
	startOutgoingProxy(net.JoinHostPort(router.Component.IpAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), proxy)

	setupMonitoring(router, config, logger)

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(router.Component.IpAddress, config.OutgoingPort, []string{uri})
	if err != nil {
		logger.Fatalf("Startup: Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	StartHeartbeats(10*time.Second, config, logger)

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			rr.UnregisterFromRouter(router.Component.IpAddress, config.OutgoingPort, []string{uri})
			router.Stop()
			break
		}
	}
}

func ParseConfig(logLevel *bool, configFile, logFilePath *string) (*Config, *gosteno.Logger, error) {
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

func MakeHashers(loggregators map[string][]string, loggregatorOutgoingPort uint32, logger *gosteno.Logger) []hasher.Hasher {
	counter := 0
	hashers := make([]hasher.Hasher, 0, len(loggregators))
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

func StartHeartbeats(ttl time.Duration, config *Config, logger *gosteno.Logger) {
	if len(config.EtcdUrls) == 0 {
		return
	}

	adapter := DefaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	adapter.Connect()

	local_ip, err := localip.LocalIP()
	if err != nil {
		panic(errors.New("StartHeartbeats: unable to resolve own IP address: " + err.Error()))
	}

	address := fmt.Sprintf("%s:%d", local_ip, config.IncomingPort)
	logger.Debugf("Starting Health Status Updates to Store: /healthstatus/trafficcontroller/%s/%s/%d", config.Zone, config.JobName, config.JobIndex)
	status, _, err := adapter.MaintainNode(storeadapter.StoreNode{
		Key:   fmt.Sprintf("/healthstatus/trafficcontroller/%s/%s/%d", config.Zone, config.JobName, config.JobIndex),
		Value: []byte(address),
		TTL:   uint64(ttl.Seconds()),
	})

	if err != nil {
		panic(err)
	}

	go func() {
		for stat := range status {
			logger.Debugf("Health updates channel pushed %v at time %v", stat, time.Now())
		}
	}()
}

func makeIncomingRouter(config *Config, logger *gosteno.Logger) *inputrouter.Router {
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
	router, err := inputrouter.NewRouter(config.Host, h, config.Config, logger)
	if err != nil {
		panic(err)
	}
	return router
}

func MakeProvider(config *Config, logger *gosteno.Logger, stopChan <-chan struct{}) outputproxy.LoggregatorServerProvider {
	var provider outputproxy.LoggregatorServerProvider
	if len(config.Loggregators) > 0 {
		logger.Debugf("Output Proxy Startup: Number of zones: %v", len(config.Loggregators))
		hashers := MakeHashers(config.Loggregators, config.LoggregatorOutgoingPort, logger)
		logger.Debugf("Output Proxy Startup: Number of hashers for the proxy: %v", len(hashers))
		provider = outputproxy.NewHashingLoggregatorServerProvider(hashers)
	} else {
		clientPool := outputproxy.NewLoggregatorClientPool(logger, int(config.LoggregatorOutgoingPort), false)
		adapter := DefaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
		adapter.Connect()
		go clientPool.RunUpdateLoop(adapter, "/healthstatus/loggregator", stopChan, EtcdQueryInterval)
		provider = outputproxy.NewDynamicLoggregatorServerProvider(clientPool)
	}
	return provider
}

func makeOutgoingProxy(config *Config, logger *gosteno.Logger) *outputproxy.Proxy {
	authorizer := authorization.NewLogAccessAuthorizer(config.ApiHost, config.SkipCertVerify)
	proxy := outputproxy.NewProxy(MakeProvider(config, logger, nil), authorizer, logger)
	return proxy
}

func startIncomingRouter(router *inputrouter.Router, logger *gosteno.Logger) {
	go router.Start(logger)
}

func setupMonitoring(router *inputrouter.Router, config *Config, logger *gosteno.Logger) {
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

func startOutgoingProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}
