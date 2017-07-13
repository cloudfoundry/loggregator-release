package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/dopplerservice"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/monitor"
	"code.cloudfoundry.org/loggregator/profiler"

	"code.cloudfoundry.org/loggregator/doppler/app"
	grpcv1 "code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/listeners"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/blacklist"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/sinkmanager"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/websocketserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/store"

	"code.cloudfoundry.org/workpool"
	gendiodes "github.com/cloudfoundry/diodes"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

const dopplerOrigin = "DopplerServer"

func main() {
	//------------------------------
	// MAIN
	//------------------------------
	rand.Seed(time.Now().UnixNano())
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	configFile := flag.String(
		"config",
		"config/doppler.json",
		"Location of the doppler config json file",
	)
	flag.Parse()

	conf, err := app.ParseConfig(*configFile)
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}

	//------------------------------
	// Monitoring
	//------------------------------
	log.Printf("Startup: Setting up the doppler server")
	err = dropsonde.Initialize(conf.MetronConfig.UDPAddress, dopplerOrigin)
	if err != nil {
		log.Fatal(err)
	}

	metricClient := setupMetricsEmitter(conf)
	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	openFileMonitor := monitor.NewLinuxFD(monitorInterval)
	uptimeMonitor := monitor.NewUptime(monitorInterval)
	batcher := initializeMetrics(conf.MetricBatchIntervalMilliseconds)

	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(conf.HealthAddr, promRegistry)
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
		conf.MaxRetainedLogMessages,
		conf.SinkSkipCertVerify,
		blacklist.New(conf.BlackListIps),
		conf.MessageDrainBufferSize,
		dopplerOrigin,
		time.Duration(conf.SinkInactivityTimeoutSeconds)*time.Second,
		time.Duration(conf.SinkIOTimeoutSeconds)*time.Second,
		time.Duration(conf.ContainerMetricTTLSeconds)*time.Second,
		time.Duration(conf.SinkDialTimeoutSeconds)*time.Second,
		batcher,
		metricClient,
		healthRegistrar,
	)

	//------------------------------
	// Ingress
	//------------------------------
	var storeAdapter storeadapter.StoreAdapter
	if !conf.DisableAnnounce || !conf.DisableSyslogDrains {
		storeAdapter = connectToEtcd(conf)
	}

	errChan := make(chan error)
	dropsondeUnmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(conf.UnmarshallerCount)

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

	udpListener, dropsondeBytesChan := listeners.NewUDPListener(
		fmt.Sprintf("%s:%d", conf.IP, conf.IncomingUDPPort),
		batcher,
		"udpListener",
	)

	grpcRouter := grpcv1.NewRouter()
	messageRouter := sinkserver.NewMessageRouter(sinkManager, grpcRouter)
	signatureVerifier := signature.NewVerifier(conf.SharedSecret)
	grpcListener, err := listeners.NewGRPCListener(
		grpcRouter,
		sinkManager,
		conf.GRPC,
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
		fmt.Sprintf("%s:%d", conf.WebsocketHost, conf.OutgoingPort),
		sinkManager,
		time.Duration(conf.WebsocketWriteTimeoutSeconds)*time.Second,
		30*time.Second,
		conf.MessageDrainBufferSize,
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
		errChan,
		dropsondeUnmarshallerCollection,
		openFileMonitor,
		uptimeMonitor,
		envelopeBuffer,
		appStoreWatcher,
		newAppServiceChan,
		deletedAppServiceChan,
		dropsondeBytesChan,
		udpListener,
		batcher,
		sinkManager,
		websocketServer,
		messageRouter,
		signatureVerifier,
		grpcListener,
		conf.DisableSyslogDrains,
	)

	log.Print("Startup: doppler server started.")

	if !conf.DisableAnnounce {
		dopplerservice.Announce(conf.IP, app.HeartbeatInterval, conf, storeAdapter)
		dopplerservice.AnnounceLegacy(conf.IP, app.HeartbeatInterval, conf, storeAdapter)
	}

	p := profiler.New(conf.PPROFPort)
	go p.Start()

	//------------------------------
	// Post Start
	//------------------------------

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)
	<-killChan
	log.Print("Shutting down")
}

func start(
	errChan chan error,
	dropsondeUnmarshallerCollection *dropsonde_unmarshaller.DropsondeUnmarshallerCollection,
	openFileMonitor *monitor.LinuxFileDescriptor,
	uptimeMonitor *monitor.Uptime,
	envelopeBuffer *diodes.ManyToOneEnvelope,
	appStoreWatcher *store.AppServiceStoreWatcher,
	newAppServiceChan <-chan store.AppService,
	deletedAppServiceChan <-chan store.AppService,
	dropsondeBytesChan <-chan []byte,
	udpListener *listeners.UDPListener,
	batcher *metricbatcher.MetricBatcher,
	sinkManager *sinkmanager.SinkManager,
	websocketServer *websocketserver.WebsocketServer,
	messageRouter *sinkserver.MessageRouter,
	signatureVerifier *signature.Verifier,
	grpcListener *listeners.GRPCListener,
	disableSyslogDrains bool,
) {
	dropsondeVerifiedBytesChan := make(chan []byte)
	go grpcListener.Start()

	go func() {
		if !disableSyslogDrains {
			appStoreWatcher.Run()
		}
	}()

	go udpListener.Start()

	udpEnvelopes := make(chan *events.Envelope)
	var nopWg sync.WaitGroup
	dropsondeUnmarshallerCollection.Run(dropsondeVerifiedBytesChan, udpEnvelopes, &nopWg)
	go func() {
		for {
			env := <-udpEnvelopes
			// metric-documentation-v1: (listeners.receivedEnvelopes) Number of envelopes
			// received from Metron on Doppler's UDP ingress listener
			batcher.BatchCounter("listeners.receivedEnvelopes").
				SetTag("protocol", "udp").
				SetTag("event_type", env.GetEventType().String()).
				Increment()
			envelopeBuffer.Set(env)
		}
	}()

	go func() {
		defer func() {
			close(dropsondeVerifiedBytesChan)
		}()
		signatureVerifier.Run(dropsondeBytesChan, dropsondeVerifiedBytesChan)
	}()

	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
	go messageRouter.Start(envelopeBuffer)
	go websocketServer.Start()
	go uptimeMonitor.Start()
	go openFileMonitor.Start()

	// The following runs forever. Put all startup functions above here.
	for err := range errChan {
		log.Printf("Got error %s", err)
	}
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

func connectToEtcd(conf *app.Config) storeadapter.StoreAdapter {
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
	if err = etcdStoreAdapter.Connect(); err != nil {
		panic(err)
	}
	return etcdStoreAdapter
}

func setupMetricsEmitter(conf *app.Config) *metricemitter.Client {
	credentials, err := plumbing.NewClientCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(conf.MetricBatchIntervalMilliseconds) * time.Millisecond

	// metric-documentation-v2: setup function
	metricClient, err := metricemitter.NewClient(
		conf.MetronConfig.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.doppler"),
		metricemitter.WithPulseInterval(batchInterval),
	)
	if err != nil {
		log.Fatalf("Could not configure metric emitter: %s", err)
	}

	return metricClient
}
