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

	_ "github.com/cloudfoundry/dropsonde/autowire"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	collectorregistrar "github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/legacycollectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/legacyproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/serveraddressprovider"
	"trafficcontroller/uaa_client"
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
	ApiHost               string
	Host                  string
	IncomingPort          uint32
	DopplerPort           uint32
	OutgoingPort          uint32
	OutgoingDropsondePort uint32
	SystemDomain          string
	SkipCertVerify        bool
	UaaHost               string
	UaaClientId           string
	UaaClientSecret       string
}

func (c *Config) setDefaults() {
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

	dopplerProxy := makeDopplerProxy(adapter, config, logger, doppler_endpoint.WebsocketHandlerProvider)
	startOutgoingDopplerProxy(net.JoinHostPort(dopplerProxy.IpAddress, strconv.FormatUint(uint64(config.OutgoingDropsondePort), 10)), dopplerProxy)

	proxy := makeLegacyProxy(adapter, config, logger)
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
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
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

func makeLegacyProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger) *legacyproxy.Proxy {
	legacyHandlerProvider := legacyproxy.NewLegacyHandlerProvider(doppler_endpoint.WebsocketHandlerProvider)
	dopplerProxy := makeDopplerProxy(adapter, config, logger, legacyHandlerProvider)

	builder := legacyproxy.NewProxyBuilder()
	builder.Handler(dopplerProxy)
	builder.Component(dopplerProxy.Component)
	builder.Logger(logger)
	builder.RequestTranslator(legacyproxy.NewRequestTranslator())
	return builder.Build()
}

func startOutgoingProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func setupMonitoring(proxy *legacyproxy.Proxy, config *Config, logger *gosteno.Logger) {
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

func makeDopplerProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger, handlerProvider doppler_endpoint.HandlerProvider) *dopplerproxy.Proxy {
	authorizer := authorization.NewLogAccessAuthorizer(config.ApiHost, config.SkipCertVerify)
	uaaClient := uaa_client.NewUaaClient(config.UaaHost, config.UaaClientId, config.UaaClientSecret, config.SkipCertVerify)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(&uaaClient)
	provider := MakeProvider(adapter, "/healthstatus/doppler", config.DopplerPort, logger)
	cgc := channel_group_connector.NewChannelGroupConnector(provider, newWebsocketListener, marshaller.DropsondeLogMessage, logger)
	proxy := dopplerproxy.NewDopplerProxy(authorizer, adminAuthorizer, handlerProvider, cgc, config.Config, logger)
	return proxy
}

func startOutgoingDopplerProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func newWebsocketListener() listener.Listener {
	messageConverter := func(message []byte) []byte {
		return message
	}
	return listener.NewWebsocket(marshaller.DropsondeLogMessage, messageConverter)
}
