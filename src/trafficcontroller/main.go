package main

import (
	"doppler/dopplerservice"
	"errors"
	"flag"
	"fmt"
	"logger"
	"monitor"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"plumbing"
	"signalmanager"
	"strconv"
	"time"
	"trafficcontroller/accesslogger"
	"trafficcontroller/authorization"
	"trafficcontroller/config"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/grpcconnector"
	"trafficcontroller/httpsetup"
	"trafficcontroller/middleware"
	"trafficcontroller/uaa_client"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_sender"
	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/cloudfoundry/dropsonde/log_sender"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

const (
	handshakeTimeout     = 5 * time.Second
	defaultBatchInterval = 1 * time.Second
	statsInterval        = 10 * time.Second
)

var (
	logFilePath          = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel             = flag.Bool("debug", false, "Debug logging")
	disableAccessControl = flag.Bool("disableAccessControl", false, "always all access to app logs")
	configFile           = flag.String("config", "config/loggregator_trafficcontroller.json", "Location of the loggregator trafficcontroller config json file")
)

func main() {
	flag.Parse()

	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	httpsetup.SetInsecureSkipVerify(conf.SkipCertVerify)

	ipAddress, err := localip.LocalIP()
	if err != nil {
		panic(fmt.Errorf("Unable to resolve own IP address: %s", err))
	}

	log := logger.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller", conf.Syslog)
	log.Info("Startup: Setting up the loggregator traffic controller")

	batcher, err := initializeMetrics("LoggregatorTrafficController", net.JoinHostPort(conf.MetronHost, strconv.Itoa(conf.MetronPort)))
	if err != nil {
		log.Errorf("Error initializing dropsonde: %s", err)
	}

	go func() {
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", conf.PPROFPort), nil)
		if err != nil {
			log.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	uptimeMonitor := monitor.NewUptime(monitorInterval)
	go uptimeMonitor.Start()
	defer uptimeMonitor.Stop()

	openFileMonitor := monitor.NewLinuxFD(monitorInterval, log)
	go openFileMonitor.Start()
	defer openFileMonitor.Stop()

	etcdAdapter := defaultStoreAdapterProvider(conf)
	err = etcdAdapter.Connect()
	if err != nil {
		panic(fmt.Errorf("Unable to connect to ETCD: %s", err))
	}

	logAuthorizer := authorization.NewLogAccessAuthorizer(*disableAccessControl, conf.ApiHost)

	uaaClient := uaa_client.NewUaaClient(conf.UaaHost, conf.UaaClient, conf.UaaClientSecret)
	adminAuthorizer := authorization.NewAdminAccessAuthorizer(*disableAccessControl, &uaaClient)

	// TODO: The preferredProtocol of udp tells the finder to pull out the Doppler URLs from the legacy ETCD endpoint.
	// Eventually we'll have a separate websocket client pool
	finder := dopplerservice.NewFinder(etcdAdapter, int(conf.DopplerPort), int(conf.GRPC.Port), []string{"ws"}, "", log)
	finder.Start()

	var accessMiddleware func(middleware.HttpHandler) *middleware.AccessHandler
	if conf.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(conf.SecurityEventLog, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			panic(fmt.Errorf("Unable to open access log: %s", err))
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()
		accessLogger := accesslogger.New(accessLog, log)
		accessMiddleware = middleware.Access(accessLogger, ipAddress, conf.OutgoingDropsondePort, log)
	}

	tlsConf, err := plumbing.NewTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		panic(fmt.Errorf("Unable to create gRPC TLS config: %s", err))
	}
	pool := grpcconnector.NewPool(20, tlsConf)
	grpcConnector := grpcconnector.New(1000, pool, finder, batcher)

	dopplerHandler := http.Handler(dopplerproxy.NewDopplerProxy(logAuthorizer, adminAuthorizer, grpcConnector, "doppler."+conf.SystemDomain, log, 15*time.Second))
	if accessMiddleware != nil {
		dopplerHandler = accessMiddleware(dopplerHandler)
	}
	startOutgoingProxy(net.JoinHostPort(ipAddress, strconv.FormatUint(uint64(conf.OutgoingDropsondePort), 10)), dopplerHandler)

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

func setupDefaultEmitter(origin, destination string) error {
	if origin == "" {
		return errors.New("Cannot initialize metrics with an empty origin")
	}

	if destination == "" {
		return errors.New("Cannot initialize metrics with an empty destination")
	}

	udpEmitter, err := emitter.NewUdpEmitter(destination)
	if err != nil {
		return fmt.Errorf("Failed to initialize dropsonde: %v", err.Error())
	}

	dropsonde.DefaultEmitter = emitter.NewEventEmitter(udpEmitter, origin)
	return nil
}

func initializeMetrics(origin, destination string) (*metricbatcher.MetricBatcher, error) {
	err := setupDefaultEmitter(origin, destination)
	if err != nil {
		// Legacy holdover.  We would prefer to panic, rather than just throwing our metrics
		// away and pretending we're running fine, but for now, we just don't want to break
		// anything.
		dropsonde.DefaultEmitter = &dropsonde.NullEventEmitter{}
	}

	// Copied from dropsonde.initialize(), since we stopped using dropsonde.Initialize
	// but needed it to continue operating the same.
	sender := metric_sender.NewMetricSender(dropsonde.DefaultEmitter)
	batcher := metricbatcher.New(sender, defaultBatchInterval)
	metrics.Initialize(sender, batcher)
	logs.Initialize(log_sender.NewLogSender(dropsonde.DefaultEmitter, gosteno.NewLogger("dropsonde/logs")))
	envelopes.Initialize(envelope_sender.NewEnvelopeSender(dropsonde.DefaultEmitter))
	go runtime_stats.NewRuntimeStats(dropsonde.DefaultEmitter, statsInterval).Run(nil)
	http.DefaultTransport = dropsonde.InstrumentedRoundTripper(http.DefaultTransport)
	return batcher, err
}

func defaultStoreAdapterProvider(conf *config.Config) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(conf.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.EtcdUrls,
	}
	if conf.EtcdRequireTLS {
		options.IsSSL = true
		options.CertFile = conf.EtcdTLSClientConfig.CertFile
		options.KeyFile = conf.EtcdTLSClientConfig.KeyFile
		options.CAFile = conf.EtcdTLSClientConfig.CAFile
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
