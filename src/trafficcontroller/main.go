package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
	"trafficcontroller/authorization"

	"common/monitor"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/config"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/profiler"
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

const EtcdQueryInterval = 5 * time.Second

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

	config, logger, err := config.ParseConfig(logLevel, configFile, logFilePath)
	if err != nil {
		panic(err)
	}

	profiler := profiler.NewProfiler(*cpuprofile, *memprofile, 1*time.Second, logger)
	profiler.Profile()
	defer profiler.Stop()

	uptimeMonitor := monitor.NewUptimeMonitor(time.Duration(config.MonitorIntervalSeconds) * time.Second)
	go uptimeMonitor.Start()
	defer uptimeMonitor.Stop()

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

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			break
		}
	}
}

func startOutgoingProxy(host string, proxy http.Handler) {
	go func() {
		err := http.ListenAndServe(host, proxy)
		if err != nil {
			panic(err)
		}
	}()
}

func makeDopplerProxy(adapter storeadapter.StoreAdapter, config *config.Config, logger *gosteno.Logger) *dopplerproxy.Proxy {
	return makeProxy(adapter, config, logger, marshaller.DropsondeLogMessage, dopplerproxy.TranslateFromDropsondePath, newDropsondeWebsocketListener, "doppler."+config.SystemDomain)
}

func makeLegacyProxy(adapter storeadapter.StoreAdapter, config *config.Config, logger *gosteno.Logger) *dopplerproxy.Proxy {
	return makeProxy(adapter, config, logger, marshaller.LoggregatorLogMessage, dopplerproxy.TranslateFromLegacyPath, newLegacyWebsocketListener, "loggregator."+config.SystemDomain)
}

func makeProxy(adapter storeadapter.StoreAdapter, config *config.Config, logger *gosteno.Logger, messageGenerator marshaller.MessageGenerator, translator dopplerproxy.RequestTranslator, listenerConstructor channel_group_connector.ListenerConstructor, cookieDomain string) *dopplerproxy.Proxy {
	logAuthorizer := authorization.NewLogAccessAuthorizer(*disableAccessControl, config.ApiHost, config.SkipCertVerify)

	uaaClient := uaa_client.NewUaaClient(config.UaaHost, config.UaaClientId, config.UaaClientSecret, config.SkipCertVerify)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(*disableAccessControl, &uaaClient)

	loggregatorServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler", logger)
	provider := serveraddressprovider.NewDynamicServerAddressProvider(loggregatorServerAddressList, config.DopplerPort, EtcdQueryInterval)
	provider.Start()

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
