package app

import (
	"log"
	"net"
	"time"

	gendiodes "code.cloudfoundry.org/diodes"
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/doppler/internal/server"
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v2"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Doppler routes envelopes from producers to any subscribers.
type Doppler struct {
	c              *Config
	healthListener net.Listener
	server         *server.Server
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
	healthRegistrar := initHealthRegistrar(promRegistry)

	//------------------------------
	// In memory store of
	// - recent logs
	// - container metrics
	//------------------------------
	sinkManager := sinks.NewSinkManager(
		d.c.MaxRetainedLogMessages,
		d.c.SinkSkipCertVerify,
		sinks.NewBlackListManager(d.c.BlackListIps),
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
	droppedMetric := metricClient.NewCounter(
		"dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	v1Buf := diodes.NewManyToOneEnvelope(10000, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Shed %d envelopes (v1 buffer)", missed)

		// metric-documentation-v1: (doppler.shedEnvelopes) Number of envelopes dropped by the
		// diode inbound from metron
		metricBatcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))
	}))

	v2Buf := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Dropped %d envelopes (v2 buffer)", missed)

		// metric-documentation-v2: (loggregator.doppler.dropped) Number of envelopes dropped by the
		// diode inbound from metron
		droppedMetric.Increment(uint64(missed))
	}))

	v1Ingress := v1.NewIngestorServer(
		v1Buf,
		v2Buf,
		metricBatcher,
		healthRegistrar,
	)
	v1Router := v1.NewRouter()
	v1Egress := v1.NewDopplerServer(
		v1Router,
		sinkManager,
		metricClient,
		healthRegistrar,
		time.Second,
		100,
	)
	v2Ingress := v2.NewIngressServer(
		v1Buf,
		v2Buf,
		metricBatcher,
		metricClient,
		healthRegistrar,
	)
	v2PubSub := v2.NewPubSub()
	v2Egress := v2.NewEgressServer(
		v2PubSub,
		healthRegistrar,
		time.Second,
		100,
	)

	var opts []plumbing.ConfigOption
	if len(d.c.GRPC.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(d.c.GRPC.CipherSuites))
	}
	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		d.c.GRPC.CertFile,
		d.c.GRPC.KeyFile,
		d.c.GRPC.CAFile,
		opts...,
	)
	if err != nil {
		log.Panicf("Failed to create Doppler server: %s", err)
	}
	srv, err := server.NewServer(
		d.c.GRPC.Port,
		v1Ingress,
		v1Egress,
		v2Ingress,
		v2Egress,
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		log.Panicf("Failed to create Doppler server: %s", err)
	}

	d.server = srv
	d.addrs.GRPC = d.server.Addr()

	//------------------------------
	// Service Discovery and Syslog Drains (etcd)
	//------------------------------
	var storeAdapter storeadapter.StoreAdapter
	if !d.c.DisableAnnounce || !d.c.DisableSyslogDrains {
		storeAdapter = connectToEtcd(d.c)
	}
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := sinks.NewAppServiceStoreWatcher(
		storeAdapter,
		sinks.NewAppServiceCache(),
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

	messageRouter := sinks.NewMessageRouter(sinkManager, v1Router)
	go messageRouter.Start(v1Buf)

	repeater := v2.NewRepeater(v2PubSub.Publish, v2Buf.Next)
	go repeater.Start()

	go d.server.Start()

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
	d.server.Stop()
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

func initHealthRegistrar(r prometheus.Registerer) *healthendpoint.Registrar {
	return healthendpoint.New(r, map[string]prometheus.Gauge{
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
}
