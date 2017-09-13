package app

import (
	"fmt"
	"log"
	"time"

	gendiodes "code.cloudfoundry.org/diodes"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/doppler/app/config"
	grpcv1 "code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/listeners"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/blacklist"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/sinkmanager"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/websocketserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/store"
	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
)

type Doppler struct {
	c *config.Config
}

func NewDoppler(c *config.Config) *Doppler {
	return &Doppler{
		c: c,
	}
}

func (d *Doppler) Start() {
	//------------------------------
	// Monitoring
	//------------------------------
	dopplerOrigin := "DopplerServer"
	log.Printf("Startup: Setting up the doppler server")
	err := dropsonde.Initialize(d.c.MetronConfig.UDPAddress, dopplerOrigin)
	if err != nil {
		log.Fatal(err)
	}

	metricClient := setupMetricsEmitter(d.c)
	batcher := initializeMetrics(d.c.MetricBatchIntervalMilliseconds)

	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(d.c.HealthAddr, promRegistry)
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
	// Caching
	//------------------------------
	sinkManager := sinkmanager.New(
		d.c.MaxRetainedLogMessages,
		d.c.SinkSkipCertVerify,
		blacklist.New(d.c.BlackListIps),
		d.c.MessageDrainBufferSize,
		dopplerOrigin,
		time.Duration(d.c.SinkInactivityTimeoutSeconds)*time.Second,
		time.Duration(d.c.SinkIOTimeoutSeconds)*time.Second,
		time.Duration(d.c.ContainerMetricTTLSeconds)*time.Second,
		time.Duration(d.c.SinkDialTimeoutSeconds)*time.Second,
		batcher,
		metricClient,
		healthRegistrar,
	)

	//------------------------------
	// Ingress
	//------------------------------
	var storeAdapter storeadapter.StoreAdapter
	if !d.c.DisableAnnounce || !d.c.DisableSyslogDrains {
		storeAdapter = connectToEtcd(d.c)
	}

	droppedMetric := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	envelopeBuffer := diodes.NewManyToOneEnvelope(10000, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Shed %d envelopes", missed)
		// metric-documentation-v1: (doppler.shedEnvelopes) Number of envelopes dropped by the
		// diode inbound from metron
		batcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))

		// metric-documentation-v2: (loggregator.doppler.dropped) Number of envelopes dropped by the
		// diode inbound from metron
		droppedMetric.Increment(uint64(missed))
	}))

	grpcRouter := grpcv1.NewRouter()
	messageRouter := sinkserver.NewMessageRouter(sinkManager, grpcRouter)
	grpcListener, err := listeners.NewGRPCListener(
		grpcRouter,
		sinkManager,
		d.c.GRPC,
		envelopeBuffer,
		batcher,
		metricClient,
		healthRegistrar,
	)
	if err != nil {
		log.Panicf("Failed to create grpcListener: %s", err)
	}

	//------------------------------
	// Egress
	//------------------------------
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(
		storeAdapter,
		store.NewAppServiceCache(),
	)

	websocketServer, err := websocketserver.New(
		fmt.Sprintf("%s:%d", d.c.WebsocketHost, d.c.OutgoingPort),
		sinkManager,
		time.Duration(d.c.WebsocketWriteTimeoutSeconds)*time.Second,
		30*time.Second,
		d.c.MessageDrainBufferSize,
		dopplerOrigin,
		batcher,
	)
	if err != nil {
		log.Panicf("Failed to create the websocket server: %s", err)
	}

	//------------------------------
	// Start
	//------------------------------
	go start(
		envelopeBuffer,
		appStoreWatcher,
		newAppServiceChan,
		deletedAppServiceChan,
		batcher,
		sinkManager,
		websocketServer,
		messageRouter,
		grpcListener,
		d.c.DisableSyslogDrains,
	)

	log.Print("Startup: doppler server started.")

	if !d.c.DisableAnnounce {
		dopplerservice.Announce(d.c.IP, config.HeartbeatInterval, d.c, storeAdapter)
		dopplerservice.AnnounceLegacy(d.c.IP, config.HeartbeatInterval, d.c, storeAdapter)
	}
}

func (d *Doppler) Stop() {
	// TODO: Drain
}

func start(
	envelopeBuffer *diodes.ManyToOneEnvelope,
	appStoreWatcher *store.AppServiceStoreWatcher,
	newAppServiceChan <-chan store.AppService,
	deletedAppServiceChan <-chan store.AppService,
	batcher *metricbatcher.MetricBatcher,
	sinkManager *sinkmanager.SinkManager,
	websocketServer *websocketserver.WebsocketServer,
	messageRouter *sinkserver.MessageRouter,
	grpcListener *listeners.GRPCListener,
	disableSyslogDrains bool,
) {
	go grpcListener.Start()

	go func() {
		if !disableSyslogDrains {
			appStoreWatcher.Run()
		}
	}()

	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
	go websocketServer.Start()

	messageRouter.Start(envelopeBuffer)
}

func initializeMetrics(batchIntervalMilliseconds uint) *metricbatcher.MetricBatcher {
	eventEmitter := dropsonde.AutowiredEmitter()
	metricSender := metric_sender.NewMetricSender(eventEmitter)
	metricBatcher := metricbatcher.New(
		metricSender,
		time.Duration(batchIntervalMilliseconds)*time.Millisecond,
	)
	metricBatcher.AddConsistentlyEmittedMetrics(
		"doppler.shedEnvelopes",
		"TruncatingBuffer.totalDroppedMessages",
		"listeners.totalReceivedMessageCount",
	)
	metrics.Initialize(metricSender, metricBatcher)
	return metricBatcher
}

func connectToEtcd(c *config.Config) storeadapter.StoreAdapter {
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

func setupMetricsEmitter(c *config.Config) *metricemitter.Client {
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
