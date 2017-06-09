package app

import (
	"errors"
	"fmt"
	"healthendpoint"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"code.cloudfoundry.org/loggregator/dopplerservice"
	"metricemitter"
	"monitor"
	"plumbing"
	"profiler"
	"trafficcontroller/internal/auth"
	"trafficcontroller/internal/proxy"

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
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type trafficController struct {
	conf                 *Config
	logFilePath          string
	disableAccessControl bool
	metricClient         metricemitter.MetricClient
}

// finder provides service discovery of Doppler processes
type finder interface {
	Start()
	Next() dopplerservice.Event
}

func NewTrafficController(
	c *Config,
	path string,
	disableAccessControl bool,
	metricClient metricemitter.MetricClient,
) *trafficController {
	return &trafficController{
		conf:                 c,
		logFilePath:          path,
		disableAccessControl: disableAccessControl,
		metricClient:         metricClient,
	}
}

func (t *trafficController) Start() {
	tlsConf := plumbing.NewTLSConfig(
		plumbing.WithCipherSuites(t.conf.CipherSuites),
	)
	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConf,
		DisableKeepAlives:   true,
	}
	http.DefaultClient.Transport = transport
	http.DefaultClient.Timeout = 20 * time.Second

	transport.TLSClientConfig.InsecureSkipVerify = t.conf.SkipCertVerify

	log.Print("Startup: Setting up the loggregator traffic controller")

	batcher, err := t.initializeMetrics("LoggregatorTrafficController", t.conf.MetronConfig.UDPAddress)
	if err != nil {
		log.Printf("Error initializing dropsonde: %s", err)
	}

	monitorInterval := time.Duration(t.conf.MonitorIntervalSeconds) * time.Second
	uptimeMonitor := monitor.NewUptime(monitorInterval)
	go uptimeMonitor.Start()
	defer uptimeMonitor.Stop()

	openFileMonitor := monitor.NewLinuxFD(monitorInterval)
	go openFileMonitor.Start()
	defer openFileMonitor.Stop()

	logAuthorizer := auth.NewLogAccessAuthorizer(t.disableAccessControl, t.conf.ApiHost)

	uaaClient := auth.NewUaaClient(t.conf.UaaHost, t.conf.UaaClient, t.conf.UaaClientSecret)
	adminAuthorizer := auth.NewAdminAccessAuthorizer(t.disableAccessControl, &uaaClient)

	// Start the health endpoint listener
	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(t.conf.HealthAddr, promRegistry)
	healthRegistry := healthendpoint.New(promRegistry, map[string]prometheus.Gauge{
		// metric-documentation-health: (firehoseStreamCount)
		// Number of open firehose streams
		"firehoseStreamCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "trafficcontroller",
				Name:      "firehoseStreamCount",
				Help:      "Number of open firehose streams",
			},
		),
		// metric-documentation-health: (appStreamCount)
		// Number of open app streams
		"appStreamCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "trafficcontroller",
				Name:      "appStreamCount",
				Help:      "Number of open app streams",
			},
		),
	})

	creds, err := plumbing.NewCredentials(
		t.conf.GRPC.CertFile,
		t.conf.GRPC.KeyFile,
		t.conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	var f finder
	switch {
	case len(t.conf.DopplerAddrs) > 0:
		f = plumbing.NewStaticFinder(t.conf.DopplerAddrs)
	default:
		etcdAdapter := t.defaultStoreAdapterProvider(t.conf)
		err = etcdAdapter.Connect()
		if err != nil {
			panic(fmt.Errorf("Unable to connect to ETCD: %s", err))
		}

		f = dopplerservice.NewFinder(
			etcdAdapter,
			int(t.conf.DopplerPort),
			int(t.conf.GRPC.Port),
			[]string{"ws"},
			"",
		)
	}

	f.Start()
	pool := plumbing.NewPool(20, grpc.WithTransportCredentials(creds))
	grpcConnector := plumbing.NewGRPCConnector(1000, pool, f, batcher, t.metricClient)

	dopplerHandler := http.Handler(
		proxy.NewDopplerProxy(
			logAuthorizer,
			adminAuthorizer,
			grpcConnector,
			"doppler."+t.conf.SystemDomain,
			15*time.Second,
			&metricShim{},
			healthRegistry,
		),
	)

	var accessMiddleware func(auth.HttpHandler) *auth.AccessHandler
	if t.conf.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(t.conf.SecurityEventLog, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			panic(fmt.Errorf("Unable to open access log: %s", err))
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()
		accessLogger := auth.NewAccessLogger(accessLog)
		accessMiddleware = auth.Access(accessLogger, t.conf.IP, t.conf.OutgoingDropsondePort)
	}

	if accessMiddleware != nil {
		dopplerHandler = accessMiddleware(dopplerHandler)
	}
	go log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", t.conf.OutgoingDropsondePort), dopplerHandler))

	// We start the profiler last so that we can definitively claim that we're ready for
	// connections by the time we're listening on the PPROFPort.
	p := profiler.New(t.conf.PPROFPort)
	go p.Start()

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)
	<-killChan
	log.Print("Shutting down")
}

func (t *trafficController) setupDefaultEmitter(origin, destination string) error {
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

func (t *trafficController) initializeMetrics(origin, destination string) (*metricbatcher.MetricBatcher, error) {
	err := t.setupDefaultEmitter(origin, destination)
	if err != nil {
		// Legacy holdover.  We would prefer to panic, rather than just throwing our metrics
		// away and pretending we're running fine, but for now, we just don't want to break
		// anything.
		dropsonde.DefaultEmitter = &dropsonde.NullEventEmitter{}
	}

	// Copied from dropsonde.initialize(), since we stopped using dropsonde.Initialize
	// but needed it to continue operating the same.
	sender := metric_sender.NewMetricSender(dropsonde.DefaultEmitter)
	batcher := metricbatcher.New(sender, time.Second)
	metrics.Initialize(sender, batcher)
	logs.Initialize(log_sender.NewLogSender(dropsonde.DefaultEmitter))
	envelopes.Initialize(envelope_sender.NewEnvelopeSender(dropsonde.DefaultEmitter))
	go runtime_stats.NewRuntimeStats(dropsonde.DefaultEmitter, 10*time.Second).Run(nil)
	http.DefaultTransport = dropsonde.InstrumentedRoundTripper(http.DefaultTransport)
	return batcher, err
}

func (t *trafficController) defaultStoreAdapterProvider(conf *Config) storeadapter.StoreAdapter {
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
