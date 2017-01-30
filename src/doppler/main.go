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

	"google.golang.org/grpc"

	"doppler/config"
	"doppler/dopplerservice"
	"doppler/grpcmanager/v1"
	"doppler/listeners"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"

	"diodes"
	"monitor"
	"profiler"
	"signalmanager"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

const (
	dopplerOrigin = "DopplerServer"
	tcpTimeout    = time.Minute
)

func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

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

	localIp, err := localip.LocalIP()
	if err != nil {
		log.Fatalf("Unable to resolve own IP address: %s", err)
	}

	setupMetricsEmitter(conf)

	log.Printf("Startup: Setting up the doppler server")
	dropsonde.Initialize(conf.MetronConfig.UDPAddress, dopplerOrigin)
	storeAdapter := connectToEtcd(conf)

	errChan := make(chan error)
	var wg sync.WaitGroup
	dropsondeUnmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(conf.UnmarshallerCount)
	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	openFileMonitor := monitor.NewLinuxFD(monitorInterval)
	uptimeMonitor := monitor.NewUptime(monitorInterval)
	batcher := initializeMetrics(conf.MetricBatchIntervalMilliseconds)
	envelopeBuffer := diodes.NewManyToOneEnvelope(10000, diodes.AlertFunc(func(missed int) {
		log.Printf("Shed %d envelopes", missed)
		batcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))
		metric.IncCounter("dropped") // TODO: add "egress" tag
	}))
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)
	dropsondeVerifiedBytesChan := make(chan []byte)
	udpListener, dropsondeBytesChan := listeners.NewUDPListener(
		fmt.Sprintf("%s:%d", localIp, conf.IncomingUDPPort),
		batcher,
		"udpListener",
	)
	blacklist := blacklist.New(conf.BlackListIps)
	metricTTL := time.Duration(conf.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(conf.SinkInactivityTimeoutSeconds) * time.Second
	sinkIOTimeout := time.Duration(conf.SinkIOTimeoutSeconds) * time.Second
	sinkManager := sinkmanager.New(
		conf.MaxRetainedLogMessages,
		conf.SinkSkipCertVerify,
		blacklist,
		conf.MessageDrainBufferSize,
		dopplerOrigin,
		sinkTimeout,
		sinkIOTimeout,
		metricTTL,
		time.Duration(conf.SinkDialTimeoutSeconds)*time.Second,
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
	grpcRouter := v1.NewRouter()
	messageRouter := sinkserver.NewMessageRouter(sinkManager, grpcRouter)
	signatureVerifier := signature.NewVerifier(conf.SharedSecret)
	host := localIp
	var tlsListener *listeners.TCPListener
	if conf.EnableTLSTransport {
		tlsConfig := &conf.TLSListenerConfig
		addr := fmt.Sprintf("%s:%d", host, tlsConfig.Port)
		contextName := "tlsListener"
		tlsListener, err = listeners.NewTCPListener(contextName, addr, tlsConfig, envelopeBuffer, batcher, tcpTimeout)
		if err != nil {
			log.Panicf("Failed to create TLS listener: %s", err)
		}

	}
	addr := fmt.Sprintf("%s:%d", host, conf.IncomingTCPPort)
	contextName := "tcpListener"
	tcpListener, err := listeners.NewTCPListener(contextName, addr, nil, envelopeBuffer, batcher, tcpTimeout)
	grpcListener, err := listeners.NewGRPCListener(grpcRouter, sinkManager, conf.GRPC, envelopeBuffer, batcher)
	if err != nil {
		log.Panicf("Failed to create grpcListener: %s", err)
	}

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
		dropsondeVerifiedBytesChan,
		dropsondeBytesChan,
		udpListener,
		batcher,
		sinkManager,
		websocketServer,
		messageRouter,
		signatureVerifier,
		tlsListener,
		tcpListener,
		grpcListener,
	)

	log.Print("Startup: doppler server started.")

	killChan := signalmanager.RegisterKillSignalChannel()
	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()

	releaseNodeChan := dopplerservice.Announce(localIp, config.HeartbeatInterval, conf, storeAdapter)
	legacyReleaseNodeChan := dopplerservice.AnnounceLegacy(localIp, config.HeartbeatInterval, conf, storeAdapter)

	// We start the profiler last so that we can difinitively say that we're ready for
	// connections by the time we're listening on PPROFPort.
	p := profiler.New(conf.PPROFPort)
	go p.Start()

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
				tcpListener,
				tlsListener,
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
	newAppServiceChan <-chan appservice.AppService,
	deletedAppServiceChan <-chan appservice.AppService,
	dropsondeVerifiedBytesChan chan []byte,
	dropsondeBytesChan <-chan []byte,
	udpListener *listeners.UDPListener,
	batcher *metricbatcher.MetricBatcher,
	sinkManager *sinkmanager.SinkManager,
	websocketServer *websocketserver.WebsocketServer,
	messageRouter *sinkserver.MessageRouter,
	signatureVerifier *signature.Verifier,
	tlsListener *listeners.TCPListener,
	tcpListener *listeners.TCPListener,
	grpcListener *listeners.GRPCListener,
) {
	wg.Add(7 + dropsondeUnmarshallerCollection.Size())

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

	go func() {
		defer wg.Done()
		tcpListener.Start()
	}()

	if tlsListener != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tlsListener.Start()
		}()
	}

	udpEnvelopes := make(chan *events.Envelope)
	dropsondeUnmarshallerCollection.Run(dropsondeVerifiedBytesChan, udpEnvelopes, &wg)
	go func() {
		for {
			env := <-udpEnvelopes
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
	tcpListener *listeners.TCPListener,
	tlsListener *listeners.TCPListener,
	sinkManager *sinkmanager.SinkManager,
	websocketServer *websocketserver.WebsocketServer,
	storeAdapter storeadapter.StoreAdapter,
) {
	go udpListener.Stop()
	go tcpListener.Stop()
	go tlsListener.Stop()
	go sinkManager.Stop()
	go websocketServer.Stop()
	appStoreWatcher.Stop()
	wg.Wait()

	storeAdapter.Disconnect()

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
	serverCreds := plumbing.NewCredentials(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)

	batchInterval := time.Duration(conf.MetricBatchIntervalMilliseconds) * time.Millisecond
	metric.Setup(
		metric.WithGrpcDialOpts(grpc.WithTransportCredentials(serverCreds)),
		metric.WithBatchInterval(batchInterval),
		metric.WithPrefix("loggregator"),
		metric.WithComponent("doppler"),
		metric.WithAddr(conf.MetronConfig.GRPCAddress),
	)
}
