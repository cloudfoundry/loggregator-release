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

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/pivotal-golang/localip"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/serveraddressprovider"
	"trafficcontroller/uaa_client"
)

var DefaultStoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
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
	MetronPort            int
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

	if c.EtcdMaxConcurrentRequests < 1 {
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
	logFilePath          = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel             = flag.Bool("debug", false, "Debug logging")
	disableAccessControl = flag.Bool("disableAccessControl", false, "always all access to app logs")
	configFile           = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
	cpuprofile           = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile           = flag.String("memprofile", "", "write memory profile to this file")
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

	dropsonde.Initialize("localhost:"+strconv.Itoa(config.MetronPort), "LoggregatorTrafficController")

	adapter := DefaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	adapter.Connect()

	ipAddress, err := localip.LocalIP()
	if err != nil {
		panic(err)
	}

	dopplerProxy := makeDopplerProxy(adapter, config, logger)
	startOutgoingDopplerProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingDropsondePort), 10)), dopplerProxy)

	legacyProxy := makeLegacyProxy(adapter, config, logger)
	startOutgoingProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), legacyProxy)

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := "loggregator." + config.SystemDomain
	err = rr.RegisterWithRouter(ipAddress, config.OutgoingPort, []string{uri})
	if err != nil {
		logger.Fatalf("Startup: Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	uri = "doppler." + config.SystemDomain
	err = rr.RegisterWithRouter(ipAddress, config.OutgoingDropsondePort, []string{uri})
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
			rr.UnregisterFromRouter(ipAddress, config.OutgoingPort, []string{uri})
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
	loggregatorServerAddressList.DiscoverAddresses()
	go loggregatorServerAddressList.Run(EtcdQueryInterval)

	return serveraddressprovider.NewDynamicServerAddressProvider(loggregatorServerAddressList, outgoingPort)
}

func startOutgoingProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func makeDopplerProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger) *dopplerproxy.Proxy {
	return makeProxy(adapter, config, logger, marshaller.DropsondeLogMessage, dopplerproxy.TranslateFromDropsondePath, newDropsondeWebsocketListener, "doppler."+config.SystemDomain)
}

func makeLegacyProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger) *dopplerproxy.Proxy {
	return makeProxy(adapter, config, logger, marshaller.LoggregatorLogMessage, dopplerproxy.TranslateFromLegacyPath, newLegacyWebsocketListener, "loggregator."+config.SystemDomain)
}

func makeProxy(adapter storeadapter.StoreAdapter, config *Config, logger *gosteno.Logger, messageGenerator marshaller.MessageGenerator, translator dopplerproxy.RequestTranslator, listenerConstructor channel_group_connector.ListenerConstructor, cookieDomain string) *dopplerproxy.Proxy {
	logAuthorizer := authorization.NewLogAccessAuthorizer(*disableAccessControl, config.ApiHost, config.SkipCertVerify)

	uaaClient := uaa_client.NewUaaClient(config.UaaHost, config.UaaClientId, config.UaaClientSecret, config.SkipCertVerify)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(*disableAccessControl, &uaaClient)

	provider := MakeProvider(adapter, "/healthstatus/doppler", config.DopplerPort, logger)
	cgc := channel_group_connector.NewChannelGroupConnector(provider, listenerConstructor, messageGenerator, logger)

	return dopplerproxy.NewDopplerProxy(logAuthorizer, adminAuthorizer, cgc, translator, cookieDomain, logger)
}

func startOutgoingDopplerProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func newDropsondeWebsocketListener(timeout time.Duration, logger *gosteno.Logger) listener.Listener {
	messageConverter := func(message []byte) ([]byte, error) {
		return message, nil
	}
	return listener.NewWebsocket(marshaller.DropsondeLogMessage, messageConverter, timeout, logger)
}

func newLegacyWebsocketListener(timeout time.Duration, logger *gosteno.Logger) listener.Listener {
	return listener.NewWebsocket(marshaller.LoggregatorLogMessage, marshaller.TranslateDropsondeToLegacyLogMessage, timeout, logger)
}
