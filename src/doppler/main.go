package main

import (
	"diodes"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"monitor"
	"profiler"
	"sync"
	"time"

	"doppler/config"
	"doppler/dopplerservice"
	"doppler/grpcmanager/v1"
	"doppler/listeners"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"

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
	DOPPLER_ORIGIN = "DopplerServer"
	TCPTimeout     = time.Minute
)

func main() {
	configFile := flag.String("config", "config/doppler.json", "Location of the doppler config json file")

	seed := time.Now().UnixNano()
	rand.Seed(seed)

	flag.Parse()

	localIp, err := localip.LocalIP()
	if err != nil {
		fatal("Unable to resolve own IP address: %s", err)
	}

	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		fatal("Unable to parse config: %s", err)
	}

	log.Printf("Startup: Setting up the doppler server")
	dropsonde.Initialize(conf.MetronAddress, DOPPLER_ORIGIN)
	storeAdapter := connectToEtcd(conf)

	doppler, err := New(
		localIp,
		conf,
		storeAdapter,
		conf.MessageDrainBufferSize,
		DOPPLER_ORIGIN,
		time.Duration(conf.WebsocketWriteTimeoutSeconds)*time.Second,
		time.Duration(conf.SinkDialTimeoutSeconds)*time.Second,
	)

	if err != nil {
		fatal("Failed to create doppler: %s", err)
	}

	go start(doppler)

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

			doppler.Stop()

			<-stopped
			<-legacyStopped

			return
		}
	}
}

func fatal(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

type Doppler struct {
	batcher *metricbatcher.MetricBatcher

	appStoreWatcher *store.AppServiceStoreWatcher

	errChan         chan error
	envelopeBuffer  *diodes.ManyToOneEnvelope
	udpListener     *listeners.UDPListener
	tcpListener     *listeners.TCPListener
	tlsListener     *listeners.TCPListener
	grpcListener    *listeners.GRPCListener
	sinkManager     *sinkmanager.SinkManager
	messageRouter   *sinkserver.MessageRouter
	websocketServer *websocketserver.WebsocketServer

	dropsondeUnmarshallerCollection *dropsonde_unmarshaller.DropsondeUnmarshallerCollection
	dropsondeBytesChan              <-chan []byte
	dropsondeVerifiedBytesChan      chan []byte
	signatureVerifier               *signature.Verifier

	storeAdapter storeadapter.StoreAdapter

	uptimeMonitor   *monitor.Uptime
	openFileMonitor *monitor.LinuxFileDescriptor

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	wg                                       sync.WaitGroup
}

func New(
	host string,
	conf *config.Config,
	storeAdapter storeadapter.StoreAdapter,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	websocketWriteTimeout time.Duration,
	dialTimeout time.Duration,
) (*Doppler, error) {
	doppler := &Doppler{
		storeAdapter:               storeAdapter,
		dropsondeVerifiedBytesChan: make(chan []byte),
	}

	keepAliveInterval := 30 * time.Second

	appStoreCache := cache.NewAppServiceCache()
	doppler.appStoreWatcher, doppler.newAppServiceChan, doppler.deletedAppServiceChan = store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)

	doppler.batcher = initializeMetrics(conf.MetricBatchIntervalMilliseconds)

	doppler.udpListener, doppler.dropsondeBytesChan = listeners.NewUDPListener(
		fmt.Sprintf("%s:%d", host, conf.IncomingUDPPort),
		doppler.batcher,
		"udpListener",
	)

	doppler.envelopeBuffer = diodes.NewManyToOneEnvelope(10000, doppler)

	var err error
	if conf.EnableTLSTransport {
		tlsConfig := &conf.TLSListenerConfig
		addr := fmt.Sprintf("%s:%d", host, tlsConfig.Port)
		contextName := "tlsListener"
		doppler.tlsListener, err = listeners.NewTCPListener(contextName, addr, tlsConfig, doppler.envelopeBuffer, doppler.batcher, TCPTimeout)
		if err != nil {
			return nil, err
		}
	}

	addr := fmt.Sprintf("%s:%d", host, conf.IncomingTCPPort)
	contextName := "tcpListener"
	doppler.tcpListener, err = listeners.NewTCPListener(contextName, addr, nil, doppler.envelopeBuffer, doppler.batcher, TCPTimeout)

	doppler.signatureVerifier = signature.NewVerifier(conf.SharedSecret)

	doppler.dropsondeUnmarshallerCollection = dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(conf.UnmarshallerCount)

	blacklist := blacklist.New(conf.BlackListIps)
	metricTTL := time.Duration(conf.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(conf.SinkInactivityTimeoutSeconds) * time.Second
	sinkIOTimeout := time.Duration(conf.SinkIOTimeoutSeconds) * time.Second
	doppler.sinkManager = sinkmanager.New(
		conf.MaxRetainedLogMessages,
		conf.SinkSkipCertVerify,
		blacklist,
		messageDrainBufferSize,
		dropsondeOrigin,
		sinkTimeout,
		sinkIOTimeout,
		metricTTL,
		dialTimeout,
	)

	grpcRouter := v1.NewRouter()
	doppler.grpcListener, err = listeners.NewGRPCListener(grpcRouter, doppler.sinkManager, conf.GRPC, doppler.envelopeBuffer, doppler.batcher)
	if err != nil {
		return nil, err
	}

	doppler.messageRouter = sinkserver.NewMessageRouter(doppler.sinkManager, grpcRouter)

	doppler.websocketServer, err = websocketserver.New(
		fmt.Sprintf("%s:%d", conf.WebsocketHost, conf.OutgoingPort),
		doppler.sinkManager,
		websocketWriteTimeout,
		keepAliveInterval,
		conf.MessageDrainBufferSize,
		dropsondeOrigin,
		doppler.batcher,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the websocket server: %s", err.Error())
	}

	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	doppler.openFileMonitor = monitor.NewLinuxFD(monitorInterval)
	doppler.uptimeMonitor = monitor.NewUptime(monitorInterval)

	return doppler, nil
}

func start(doppler *Doppler) {
	doppler.errChan = make(chan error)

	doppler.wg.Add(7 + doppler.dropsondeUnmarshallerCollection.Size())

	go func() {
		defer doppler.wg.Done()
		doppler.grpcListener.Start()
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.appStoreWatcher.Run()
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.udpListener.Start()
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.tcpListener.Start()
	}()

	if doppler.tlsListener != nil {
		doppler.wg.Add(1)
		go func() {
			defer doppler.wg.Done()
			doppler.tlsListener.Start()
		}()
	}

	udpEnvelopes := make(chan *events.Envelope)
	doppler.dropsondeUnmarshallerCollection.Run(doppler.dropsondeVerifiedBytesChan, udpEnvelopes, &doppler.wg)
	go func() {
		for {
			env := <-udpEnvelopes
			doppler.batcher.BatchCounter("listeners.receivedEnvelopes").
				SetTag("protocol", "udp").
				SetTag("event_type", env.GetEventType().String()).
				Increment()
			doppler.envelopeBuffer.Set(env)
		}
	}()

	go func() {
		defer func() {
			doppler.wg.Done()
			close(doppler.dropsondeVerifiedBytesChan)
		}()
		doppler.signatureVerifier.Run(doppler.dropsondeBytesChan, doppler.dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.sinkManager.Start(doppler.newAppServiceChan, doppler.deletedAppServiceChan)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.messageRouter.Start(doppler.envelopeBuffer)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.websocketServer.Start()
	}()

	go doppler.uptimeMonitor.Start()
	go doppler.openFileMonitor.Start()

	// The following runs forever. Put all startup functions above here.
	for err := range doppler.errChan {
		log.Printf("Got error %s", err)
	}
}

func (doppler *Doppler) Stop() {
	go doppler.udpListener.Stop()
	go doppler.tcpListener.Stop()
	go doppler.tlsListener.Stop()
	go doppler.sinkManager.Stop()
	go doppler.websocketServer.Stop()
	doppler.appStoreWatcher.Stop()
	doppler.wg.Wait()

	doppler.storeAdapter.Disconnect()
	close(doppler.errChan)
	doppler.uptimeMonitor.Stop()
	doppler.openFileMonitor.Stop()
}

func (doppler *Doppler) Alert(missed int) {
	log.Printf("Shed %d envelopes", missed)
	doppler.batcher.BatchCounter("doppler.shedEnvelopes").Add(uint64(missed))
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
