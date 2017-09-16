package app

import (
	"log"
	"net"
	"time"

	gendiodes "code.cloudfoundry.org/diodes"
	"code.cloudfoundry.org/loggregator/diodes"
	grpcv1 "code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/listeners"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/store"
	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

// Doppler routes envelopes from producers to any subscribers.
type Doppler struct {
	c              *Config
	healthListener net.Listener
	grpcListener   *listeners.GRPCListener
	addrs          Addrs
}

// NewLegacyDoppler creates a new Doppler with the given config.
// Using the config for construction is deprecated and will be removed once
// syslog and etcd are removed.
func NewLegacyDoppler(c *Config) *Doppler {
	return &Doppler{
		c: c,
	}
}

// NewDoppler creates a new Doppler with the given options. Each provided
// DopplerOption will manipulate the Doppler behavior.
// NOTE: This construction path does not allow for Syslog drains or announcing
// via etcd.
func NewDoppler(grpc GRPC, opts ...DopplerOption) *Doppler {
	d := &Doppler{
		c: &Config{
			GRPC: grpc,
			MetricBatchIntervalMilliseconds: 5000,
			MetronConfig: MetronConfig{
				UDPAddress:  "127.0.0.1:3457",
				GRPCAddress: "127.0.0.1:3458",
			},
			HealthAddr:                   "localhost:14825",
			MaxRetainedLogMessages:       100,
			MessageDrainBufferSize:       10000,
			SinkInactivityTimeoutSeconds: 3600,
			ContainerMetricTTLSeconds:    120,
			DisableAnnounce:              true,
			DisableSyslogDrains:          true,
		},
	}

	for _, o := range opts {
		o(d)
	}

	return d
}

// DopplerOption is used to configure a new Doppler.
type DopplerOption func(*Doppler)

// WithMetricReporting returns a DopplerOption that enables Doppler to emit
// metrics about itself.
// This option is experimental.
func WithMetricReporting() DopplerOption {
	return func(d *Doppler) {
		panic("Not yet implemented")
	}
}

// WithPersistence turns on recent logs and container metric storage.
// This option is experimental.
func WithPersistence() DopplerOption {
	return func(d *Doppler) {
		panic("Not yet implemented")
	}
}

// Start enables the Doppler to start receiving envelope, accepting
// subscriptions and routing data.
func (d *Doppler) Start() {
	log.Printf("Startup: Setting up the doppler server")

	//------------------------------
	// v1 Metrics (UDP)
	//------------------------------
	metricBatcher := initV1Metrics(
		d.c.MetricBatchIntervalMilliseconds,
		d.c.MetronConfig.UDPAddress,
	)

	//------------------------------
	// v2 Metrics (gRPC)
	//------------------------------
	metricClient := initV2Metrics(d.c)

	//------------------------------
	// Health
	//------------------------------
	promRegistry := prometheus.NewRegistry()
	d.healthListener = healthendpoint.StartServer(d.c.HealthAddr, promRegistry)
	d.addrs.Health = d.healthListener.Addr().String()
	healthRegistrar := healthendpoint.New(promRegistry, map[string]prometheus.Gauge{
		// metric-documentation-health: (ingressStreamCount)
		// Number of open firehose streams
		"ingressStreamCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "doppler",
				Name:      "ingressStreamCount",
				Help:      "Number of open ingress streams",
			},
		),
		// metric-documentation-health: (subscriptionCount)
		// Number of open subscriptions
		"subscriptionCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "doppler",
				Name:      "subscriptionCount",
				Help:      "Number of open subscriptions",
			},
		),
		// metric-documentation-health: (recentLogCacheCount)
		// Number of recent log caches
		"recentLogCacheCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "doppler",
				Name:      "recentLogCacheCount",
				Help:      "Number of recent log caches",
			},
		),
		// metric-documentation-health: (containerMetricCacheCount)
		// Number of container metric caches
		"containerMetricCacheCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "doppler",
				Name:      "containerMetricCacheCount",
				Help:      "Number of container metric caches",
			},
		),
	})

	//------------------------------
	// In memory store of
	// - recent logs
	// - container metrics
	//------------------------------
	sinkManager := sinkserver.NewSinkManager(
		d.c.MaxRetainedLogMessages,
		d.c.SinkSkipCertVerify,
		sinkserver.NewBlackListManager(d.c.BlackListIps),
		d.c.MessageDrainBufferSize,
		"DopplerServer",
		time.Duration(d.c.SinkInactivityTimeoutSeconds)*time.Second,
		time.Duration(d.c.SinkIOTimeoutSeconds)*time.Second,
		time.Duration(d.c.ContainerMetricTTLSeconds)*time.Second,
		time.Duration(d.c.SinkDialTimeoutSeconds)*time.Second,
		metricBatcher,
		metricClient,
		healthRegistrar,
	)

	//------------------------------
	// Ingress (gRPC v1 and v2)
	// Egress  (gRPC v1)
	//------------------------------
	droppedMetric := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	envelopeBuffer := diodes.NewManyToOneEnvelope(10000, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Shed %d envelopes", missed)
		// metric-documentation-v1: (doppler.shedEnvelopes) Number of envelopes dropped by the
		// diode inbound from metron
		metricBatcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))

		// metric-documentation-v2: (loggregator.doppler.dropped) Number of envelopes dropped by the
		// diode inbound from metron
		droppedMetric.Increment(uint64(missed))
	}))

	grpcRouter := grpcv1.NewRouter()
	messageRouter := sinkserver.NewMessageRouter(sinkManager, grpcRouter)
	var err error
	d.grpcListener, err = listeners.NewGRPCListener(
		grpcRouter,
		sinkManager,
		listeners.GRPCConfig{
			Port:         d.c.GRPC.Port,
			CAFile:       d.c.GRPC.CAFile,
			CertFile:     d.c.GRPC.CertFile,
			KeyFile:      d.c.GRPC.KeyFile,
			CipherSuites: d.c.GRPC.CipherSuites,
		},
		envelopeBuffer,
		metricBatcher,
		metricClient,
		healthRegistrar,
	)
	if err != nil {
		log.Panicf("Failed to create grpcListener: %s", err)
	}

	d.addrs.GRPC = d.grpcListener.Addr()

	//------------------------------
	// Service Discovery and Syslog Drains (etcd)
	//------------------------------
	var storeAdapter storeadapter.StoreAdapter
	if !d.c.DisableAnnounce || !d.c.DisableSyslogDrains {
		storeAdapter = connectToEtcd(d.c)
	}
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(
		storeAdapter,
		store.NewAppServiceCache(),
	)
	if !d.c.DisableAnnounce {
		serviceConfig := &dopplerservice.Config{
			Index:        d.c.Index,
			JobName:      d.c.JobName,
			Zone:         d.c.Zone,
			OutgoingPort: d.c.OutgoingPort,
		}
		dopplerservice.Announce(d.c.IP, HeartbeatInterval, serviceConfig, storeAdapter)
		dopplerservice.AnnounceLegacy(d.c.IP, HeartbeatInterval, serviceConfig, storeAdapter)
	}
	if !d.c.DisableSyslogDrains {
		go appStoreWatcher.Run()
	}

	//------------------------------
	// Start
	//------------------------------
	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
	go messageRouter.Start(envelopeBuffer)
	go d.grpcListener.Start()

	log.Print("Startup: doppler server started.")
}

// Addrs stores listener addresses of the Doppler process.
type Addrs struct {
	GRPC   string
	Health string
}

// Addrs returns a copy of the listeners' addresses.
func (d *Doppler) Addrs() Addrs {
	return d.addrs
}

// Stop closes the gRPC and health listeners.
func (d *Doppler) Stop() {
	// TODO: Drain
	d.healthListener.Close()
	d.grpcListener.Stop()
}

func initV1Metrics(milliseconds uint, udpAddr string) *metricbatcher.MetricBatcher {
	err := dropsonde.Initialize(udpAddr, "DopplerServer")
	if err != nil {
		log.Fatal(err)
	}
	eventEmitter := dropsonde.AutowiredEmitter()
	metricSender := metric_sender.NewMetricSender(eventEmitter)
	metricBatcher := metricbatcher.New(
		metricSender,
		time.Duration(milliseconds)*time.Millisecond,
	)
	metricBatcher.AddConsistentlyEmittedMetrics(
		"doppler.shedEnvelopes",
		"TruncatingBuffer.totalDroppedMessages",
		"listeners.totalReceivedMessageCount",
	)
	metrics.Initialize(metricSender, metricBatcher)
	return metricBatcher
}

func initV2Metrics(c *Config) *metricemitter.Client {
	credentials, err := plumbing.NewClientCredentials(
		c.GRPC.CertFile,
		c.GRPC.KeyFile,
		c.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(c.MetricBatchIntervalMilliseconds) * time.Millisecond

	// metric-documentation-v2: setup function
	metricClient, err := metricemitter.NewClient(
		c.MetronConfig.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.doppler"),
		metricemitter.WithPulseInterval(batchInterval),
	)
	if err != nil {
		log.Fatalf("Could not configure metric emitter: %s", err)
	}

	return metricClient
}

func connectToEtcd(c *Config) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(c.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: c.EtcdUrls,
	}
	if c.EtcdRequireTLS {
		options.IsSSL = true
		options.CertFile = c.EtcdTLSClientConfig.CertFile
		options.KeyFile = c.EtcdTLSClientConfig.KeyFile
		options.CAFile = c.EtcdTLSClientConfig.CAFile
	}
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}
	if err = etcdStoreAdapter.Connect(); err != nil {
		panic(err)
	}
	return etcdStoreAdapter
}
