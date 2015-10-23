package main

import (
	"doppler/dopplerservice"
	"flag"
	"logger"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"common/monitor"
	"trafficcontroller/authorization"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/config"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/profiler"
	"trafficcontroller/uaa_client"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"
)

var DefaultStoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}
	return etcdStoreAdapter
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

	config, err := config.ParseConfig(*logLevel, *configFile, *logFilePath)
	if err != nil {
		panic(err)
	}

	log := logger.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller", config.Syslog)
	log.Info("Startup: Setting up the loggregator traffic controller")

	dropsonde.Initialize("127.0.0.1:"+strconv.Itoa(config.MetronPort), "LoggregatorTrafficController")

	profiler := profiler.NewProfiler(*cpuprofile, *memprofile, 1*time.Second, log)
	profiler.Profile()
	defer profiler.Stop()

	uptimeMonitor := monitor.NewUptimeMonitor(time.Duration(config.MonitorIntervalSeconds) * time.Second)
	go uptimeMonitor.Start()
	defer uptimeMonitor.Stop()

	etcdAdapter := DefaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err = etcdAdapter.Connect()
	if err != nil {
		log.Errorf("Cannot connect to ETCD: %s", err.Error())
		os.Exit(-1)
	}

	ipAddress, err := localip.LocalIP()
	if err != nil {
		panic(err)
	}

	logAuthorizer := authorization.NewLogAccessAuthorizer(*disableAccessControl, config.ApiHost, config.SkipCertVerify)

	uaaClient := uaa_client.NewUaaClient(config.UaaHost, config.UaaClientId, config.UaaClientSecret, config.SkipCertVerify)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(*disableAccessControl, &uaaClient)

	preferredServers := func(string) bool { return false }
	finder := dopplerservice.NewLegacyFinder(etcdAdapter, int(config.DopplerPort), preferredServers, nil, log)
	finder.Start()

	dopplerProxy := makeProxy(etcdAdapter, config, log, marshaller.DropsondeLogMessage, dopplerproxy.TranslateFromDropsondePath,
		newDropsondeWebsocketListener, finder, logAuthorizer, adminAuthorizer, "doppler."+config.SystemDomain)
	startOutgoingDopplerProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingDropsondePort), 10)), dopplerProxy)

	legacyProxy := makeProxy(etcdAdapter, config, log, marshaller.LoggregatorLogMessage, dopplerproxy.TranslateFromLegacyPath,
		newLegacyWebsocketListener, finder, logAuthorizer, adminAuthorizer, "loggregator."+config.SystemDomain)
	startOutgoingProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), legacyProxy)

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	dumpChan := registerGoRoutineDumpSignalChannel()

	for {
		select {
		case <-dumpChan:
			logger.DumpGoRoutine()
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

func makeProxy(adapter storeadapter.StoreAdapter,
	config *config.Config, logger *gosteno.Logger,
	messageGenerator marshaller.MessageGenerator,
	translator dopplerproxy.RequestTranslator,
	listenerConstructor channel_group_connector.ListenerConstructor,
	finder dopplerservice.Finder,
	logAuthorizer authorization.LogAccessAuthorizer,
	adminAuthorizer authorization.AdminAccessAuthorizer,
	cookieDomain string) *dopplerproxy.Proxy {

	cgc := channel_group_connector.NewChannelGroupConnector(finder, listenerConstructor, messageGenerator, logger)
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

func registerGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}
