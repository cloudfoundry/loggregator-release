package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"metric"
	"plumbing"
	"sync"
	"time"

	"diodes"
	"doppler/config"
	"doppler/dopplerservice"
	grpcv1 "doppler/grpcmanager/v1"
	"doppler/listeners"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"doppler/store"
	"monitor"
	"profiler"
	"signalmanager"

	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"google.golang.org/grpc"
)

const dopplerOrigin = "DopplerServer"

func main() {
	//------------------------------
	// MAIN
	//------------------------------
	rand.Seed(time.Now().UnixNano())

	configFile := flag.String(
		"config",
		"config/doppler.json",
		"Location of the doppler config json file",
	)
	flag.Parse()

	conf, err := config.ParseConfig(*configFile)
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

	setupMetricsEmitter(conf)
	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	openFileMonitor := monitor.NewLinuxFD(monitorInterval)
	uptimeMonitor := monitor.NewUptime(monitorInterval)

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
	)

	//------------------------------
	// Ingress
	//------------------------------
	storeAdapter := connectToEtcd(conf)

	errChan := make(chan error)
	var wg sync.WaitGroup
	dropsondeUnmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(conf.UnmarshallerCount)
	batcher := initializeMetrics(conf.MetricBatchIntervalMilliseconds)
	envelopeBuffer := diodes.NewManyToOneEnvelope(10000, diodes.AlertFunc(func(missed int) {
		log.Printf("Shed %d envelopes", missed)
		// metric-documentation-v1: (doppler.shedEnvelopes) Number of envelopes dropped by the
		// diode inbound from metron
		batcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))
		// metric-documentation-v2: (loggregator.doppler.dropped) Number of envelopes dropped by the
		// diode inbound from metron
		metric.IncCounter("dropped",
			metric.WithIncrement(uint64(missed)),
			metric.WithVersion(2, 0),
			metric.WithTag("direction", "ingress"),
		)
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
		wg,
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
	)

	log.Print("Startup: doppler server started.")

	killChan := signalmanager.RegisterKillSignalChannel()
	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()

	releaseNodeChan := dopplerservice.Announce(conf.IP, config.HeartbeatInterval, conf, storeAdapter)
	legacyReleaseNodeChan := dopplerservice.AnnounceLegacy(conf.IP, config.HeartbeatInterval, conf, storeAdapter)

	p := profiler.New(conf.PPROFPort)
	go p.Start()

	//------------------------------
	// Post Start
	//------------------------------

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Print("Shutting down")

			stopped := make(chan bool)
			legacyStopped := make(chan bool)
			releaseNodeChan <- stopped
			legacyReleaseNodeChan <- legacyStopped

			stop(
				errChan,
				wg,
				openFileMonitor,
				uptimeMonitor,
				appStoreWatcher,
				udpListener,
				sinkManager,
				websocketServer,
				storeAdapter,
			)

			<-stopped
			<-legacyStopped

			return
		}
	}
}

func start(
	errChan chan error,
	wg sync.WaitGroup,
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
) {
	wg.Add(7 + dropsondeUnmarshallerCollection.Size())

	dropsondeVerifiedBytesChan := make(chan []byte)

	go func() {
		defer wg.Done()
		grpcListener.Start()
	}()

	go func() {
		defer wg.Done()
		appStoreWatcher.Run()
	}()

	go func() {
		defer wg.Done()
		udpListener.Start()
	}()

	udpEnvelopes := make(chan *events.Envelope)
	dropsondeUnmarshallerCollection.Run(dropsondeVerifiedBytesChan, udpEnvelopes, &wg)
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
			wg.Done()
			close(dropsondeVerifiedBytesChan)
		}()
		signatureVerifier.Run(dropsondeBytesChan, dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer wg.Done()
		sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
	}()

	go func() {
		defer wg.Done()
		messageRouter.Start(envelopeBuffer)
	}()

	go func() {
		defer wg.Done()
		websocketServer.Start()
	}()

	go uptimeMonitor.Start()
	go openFileMonitor.Start()

	// The following runs forever. Put all startup functions above here.
	for err := range errChan {
		log.Printf("Got error %s", err)
	}
}

func stop(
	errChan chan error,
	wg sync.WaitGroup,
	openFileMonitor *monitor.LinuxFileDescriptor,
	uptimeMonitor *monitor.Uptime,
	appStoreWatcher *store.AppServiceStoreWatcher,
	udpListener *listeners.UDPListener,
	sinkManager *sinkmanager.SinkManager,
	websocketServer *websocketserver.WebsocketServer,
	storeAdapter storeadapter.StoreAdapter,
) {
	go udpListener.Stop()
	go sinkManager.Stop()
	go websocketServer.Stop()
	appStoreWatcher.Stop()
	wg.Wait()

	err := storeAdapter.Disconnect()
	if err != nil {
		log.Printf("error when disconnecting from store adapter: %s", err)
	}
	close(errChan)

	uptimeMonitor.Stop()
	openFileMonitor.Stop()
}

func initializeMetrics(batchIntervalMilliseconds uint) *metricbatcher.MetricBatcher {
	eventEmitter := dropsonde.AutowiredEmitter()
	metricSender := metric_sender.NewMetricSender(eventEmitter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(batchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)
	return metricBatcher
}

func connectToEtcd(conf *config.Config) storeadapter.StoreAdapter {
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

func setupMetricsEmitter(conf *config.Config) {
	serverCreds, err := plumbing.NewCredentials(
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
	metric.Setup(
		metric.WithGrpcDialOpts(grpc.WithTransportCredentials(serverCreds)),
		metric.WithBatchInterval(batchInterval),
		metric.WithOrigin("loggregator.doppler"),
		metric.WithAddr(conf.MetronConfig.GRPCAddress),
		metric.WithDeploymentMeta(conf.DeploymentName, conf.JobName, conf.Index),
	)
}
