package main

import (
	"crypto/tls"
	"doppler/dopplerservice"
	"flag"
	"fmt"
	"logger"
	"monitor"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"signalmanager"
	"strconv"
	"time"
	"trafficcontroller/accesslogger"
	"trafficcontroller/authorization"
	"trafficcontroller/channel_group_connector"
	"trafficcontroller/config"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/middleware"
	"trafficcontroller/uaa_client"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"
)

const (
	pprofPort        = "6060"
	handshakeTimeout = 5 * time.Second
)

var (
	logFilePath          = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel             = flag.Bool("debug", false, "Debug logging")
	disableAccessControl = flag.Bool("disableAccessControl", false, "always all access to app logs")
	configFile           = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
)

func main() {
	flag.Parse()

	config, err := config.ParseConfig(*logLevel, *configFile)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipCertVerify}}
	http.DefaultClient.Transport = transport

	ipAddress, err := localip.LocalIP()
	if err != nil {
		panic(fmt.Errorf("Unable to resolve own IP address: %s", err))
	}

	log := logger.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller", config.Syslog)
	log.Info("Startup: Setting up the loggregator traffic controller")

	dropsonde.Initialize(net.JoinHostPort(config.MetronHost, strconv.Itoa(config.MetronPort)), "LoggregatorTrafficController")

	go func() {
		err := http.ListenAndServe(net.JoinHostPort(ipAddress, pprofPort), nil)
		if err != nil {
			log.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	uptimeMonitor := monitor.NewUptimeMonitor(time.Duration(config.MonitorIntervalSeconds) * time.Second)
	go uptimeMonitor.Start()
	defer uptimeMonitor.Stop()

	etcdAdapter := defaultStoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err = etcdAdapter.Connect()
	if err != nil {
		panic(fmt.Errorf("Unable to connect to ETCD: %s", err))
	}

	logAuthorizer := authorization.NewLogAccessAuthorizer(*disableAccessControl, config.ApiHost)

	uaaClient := uaa_client.NewUaaClient(config.UaaHost, config.UaaClientId, config.UaaClientSecret)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(*disableAccessControl, &uaaClient)

	// TODO: The preferredProtocol of udp tells the finder to pull out the Doppler URLs from the legacy ETCD endpoint.
	// Eventually we'll have a separate websocket client pool
	finder := dopplerservice.NewFinder(etcdAdapter, int(config.DopplerPort), []string{"udp"}, "", log)
	finder.Start()
	// Draining the finder's events channel in order to not block the finder from handling etcd events.
	go func() {
		for {
			finder.Next()
		}
	}()

	var accessMiddleware, legacyAccessMiddleware func(middleware.HttpHandler) *middleware.AccessHandler
	if config.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(config.SecurityEventLog, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			panic(fmt.Errorf("Unable to open access log: %s", err))
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()
		accessLogger := accesslogger.New(accessLog, log)
		accessMiddleware = middleware.Access(accessLogger, ipAddress, config.OutgoingDropsondePort, log)
		legacyAccessMiddleware = middleware.Access(accessLogger, ipAddress, config.OutgoingPort, log)
	}

	dopplerCgc := channel_group_connector.NewChannelGroupConnector(finder, newDropsondeWebsocketListener, marshaller.DropsondeLogMessage, log)
	dopplerHandler := http.Handler(dopplerproxy.NewDopplerProxy(logAuthorizer, adminAuthorizer, dopplerCgc, dopplerproxy.TranslateFromDropsondePath, "doppler."+config.SystemDomain, log))
	if accessMiddleware != nil {
		dopplerHandler = accessMiddleware(dopplerHandler)
	}
	startOutgoingProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingDropsondePort), 10)), dopplerHandler)

	legacyCgc := channel_group_connector.NewChannelGroupConnector(finder, newLegacyWebsocketListener, marshaller.LoggregatorLogMessage, log)
	legacyHandler := http.Handler(dopplerproxy.NewDopplerProxy(logAuthorizer, adminAuthorizer, legacyCgc, dopplerproxy.TranslateFromLegacyPath, "loggregator."+config.SystemDomain, log))
	if legacyAccessMiddleware != nil {
		legacyHandler = legacyAccessMiddleware(legacyHandler)
	}
	startOutgoingProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(config.OutgoingPort), 10)), legacyHandler)

	killChan := signalmanager.RegisterKillSignalChannel()
	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Info("Shutting down")
			return
		}
	}
}

func defaultStoreAdapterProvider(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
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

func startOutgoingProxy(host string, proxy http.Handler) {
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
	return listener.NewWebsocket(marshaller.DropsondeLogMessage, messageConverter, timeout, handshakeTimeout, logger)
}

func newLegacyWebsocketListener(timeout time.Duration, logger *gosteno.Logger) listener.Listener {
	return listener.NewWebsocket(marshaller.LoggregatorLogMessage, marshaller.TranslateDropsondeToLegacyLogMessage, timeout, handshakeTimeout, logger)
}
