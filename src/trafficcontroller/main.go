package main

import (
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"time"
	"trafficcontroller/authorization"
	"trafficcontroller/outputproxy"

	_ "github.com/cloudfoundry/dropsonde/autowire"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/dropsondeproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/serveraddressprovider"
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
	LoggregatorIncomingPort uint32
	LoggregatorOutgoingPort uint32
	IncomingPort            uint32
	OutgoingPort            uint32
	OutgoingDropsondePort   uint32
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

	adapter := DefaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	adapter.Connect()

	dropsondeProxy := makeDropsondeProxy(adapter, config, logger)
	startOutgoingDropsondeProxy(net.JoinHostPort(dropsondeProxy.IpAddress, strconv.FormatUint(uint64(config.OutgoingDropsondePort), 10)), dropsondeProxy)

	proxy := makeLoggregatorProxy(adapter, config, logger)
	startOutgoingProxy(net.JoinHostPort(proxy.IpAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), proxy)

	setupMonitoring(proxy, config, logger)

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(proxy.IpAddress, config.OutgoingPort, []string{uri})
	if err != nil {
		logger.Fatalf("Startup: Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	uri = "doppler." + config.SystemDomain
	err = rr.RegisterWithRouter(proxy.IpAddress, config.OutgoingDropsondePort, []string{uri})
	if err != nil {
		logger.Fatalf("Startup: Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}


	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			rr.UnregisterFromRouter(proxy.IpAddress, config.OutgoingPort, []string{uri})
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

	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to regsiter component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
			return fakeyagnats.New(), nil
		}
	}

	err = config.validate(logger)
	if err != nil {
		return nil, nil, err
	}
	return config, logger, nil
}

func MakeProvider(adapter storeadapter.StoreAdapter, storeKeyPrefix string, outgoingPort uint32, logger *gosteno.Logger) serveraddressprovider.ServerAddressProvider {
	loggregatorServerAddressList := servicediscovery.NewServerAddressList(adapter, storeKeyPrefix, logger)
	go loggregatorServerAddressList.Run(EtcdQueryInterval)

	return serveraddressprovider.NewDynamicServerAddressProvider(loggregatorServerAddressList, outgoingPort)
}

func makeLoggregatorProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger) *outputproxy.Proxy {
	authorizer := authorization.NewLogAccessAuthorizer(config.ApiHost, config.SkipCertVerify)
	provider := MakeProvider(adapter, "/healthstatus/loggregator", config.LoggregatorOutgoingPort, logger)
	proxy := outputproxy.NewProxy(provider, authorizer, config.Config, logger)
	return proxy
}

func startOutgoingProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func setupMonitoring(proxy *outputproxy.Proxy, config *Config, logger *gosteno.Logger) {
	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err := cr.RegisterWithCollector(proxy.Component)
	if err != nil {
		panic(err)
	}

	go func() {
		err := proxy.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()
}

func makeDropsondeProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger) *dropsondeproxy.Proxy {
	authorizer := authorization.NewLogAccessAuthorizer(config.ApiHost, config.SkipCertVerify)
	provider := MakeProvider(adapter, "/healthstatus/doppler", config.OutgoingDropsondePort, logger)
	cgc := channel_group_connector.NewChannelGroupConnector(provider, newWebsocketListener, marshaller.DropsondeLogMessage, logger)
	proxy := dropsondeproxy.NewDropsondeProxy(authorizer, dropsondeproxy.DefaultHandlerProvider, cgc, config.Config, logger)
	return proxy
}

func startOutgoingDropsondeProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func newWebsocketListener() listener.Listener {
	return listener.NewWebsocket(marshaller.DropsondeLogMessage)
}
