package main

import (
	"fmt"
	"sync"
	"time"

	"doppler/config"
	"doppler/grpcmanager"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"

	"doppler/listeners"
	"monitor"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
)

type Doppler struct {
	*gosteno.Logger
	batcher *metricbatcher.MetricBatcher

	appStoreWatcher *store.AppServiceStoreWatcher

	errChan         chan error
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
	envelopeChan                    chan *events.Envelope
	signatureVerifier               *signature.Verifier

	storeAdapter storeadapter.StoreAdapter

	uptimeMonitor   *monitor.Uptime
	openFileMonitor *monitor.LinuxFileDescriptor

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	wg                                       sync.WaitGroup
}

func New(
	logger *gosteno.Logger,
	host string,
	conf *config.Config,
	storeAdapter storeadapter.StoreAdapter,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	websocketWriteTimeout time.Duration,
	dialTimeout time.Duration,
) (*Doppler, error) {
	doppler := &Doppler{
		Logger:                     logger,
		storeAdapter:               storeAdapter,
		dropsondeVerifiedBytesChan: make(chan []byte),
	}

	keepAliveInterval := 30 * time.Second

	appStoreCache := cache.NewAppServiceCache()
	doppler.appStoreWatcher, doppler.newAppServiceChan, doppler.deletedAppServiceChan = store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache, logger)

	doppler.batcher = initializeMetrics(conf.MetricBatchIntervalMilliseconds)

	doppler.envelopeChan = make(chan *events.Envelope)

	doppler.udpListener, doppler.dropsondeBytesChan = listeners.NewUDPListener(
		fmt.Sprintf("%s:%d", host, conf.IncomingUDPPort),
		doppler.batcher,
		logger,
		"udpListener",
	)

	var err error
	if conf.EnableTLSTransport {
		tlsConfig := &conf.TLSListenerConfig
		addr := fmt.Sprintf("%s:%d", host, tlsConfig.Port)
		contextName := "tlsListener"
		doppler.tlsListener, err = listeners.NewTCPListener(contextName, addr, tlsConfig, doppler.envelopeChan, doppler.batcher, TCPTimeout, logger)
		if err != nil {
			return nil, err
		}
	}

	addr := fmt.Sprintf("%s:%d", host, conf.IncomingTCPPort)
	contextName := "tcpListener"
	doppler.tcpListener, err = listeners.NewTCPListener(contextName, addr, nil, doppler.envelopeChan, doppler.batcher, TCPTimeout, logger)

	doppler.signatureVerifier = signature.NewVerifier(logger, conf.SharedSecret)

	doppler.dropsondeUnmarshallerCollection = dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(logger, conf.UnmarshallerCount)

	blacklist := blacklist.New(conf.BlackListIps, logger)
	metricTTL := time.Duration(conf.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(conf.SinkInactivityTimeoutSeconds) * time.Second
	sinkIOTimeout := time.Duration(conf.SinkIOTimeoutSeconds) * time.Second
	doppler.sinkManager = sinkmanager.New(
		conf.MaxRetainedLogMessages,
		conf.SinkSkipCertVerify,
		blacklist,
		logger,
		messageDrainBufferSize,
		dropsondeOrigin,
		sinkTimeout,
		sinkIOTimeout,
		metricTTL,
		dialTimeout,
	)

	grpcRouter := grpcmanager.NewRouter()
	doppler.grpcListener, err = listeners.NewGRPCListener(grpcRouter, doppler.sinkManager, conf.GRPC, doppler.envelopeChan)
	if err != nil {
		return nil, err
	}

	doppler.messageRouter = sinkserver.NewMessageRouter(logger, doppler.sinkManager, grpcRouter)

	doppler.websocketServer, err = websocketserver.New(
		fmt.Sprintf("%s:%d", conf.WebsocketHost, conf.OutgoingPort),
		doppler.sinkManager,
		websocketWriteTimeout,
		keepAliveInterval,
		conf.MessageDrainBufferSize,
		dropsondeOrigin,
		doppler.batcher,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the websocket server: %s", err.Error())
	}

	monitorInterval := time.Duration(conf.MonitorIntervalSeconds) * time.Second
	doppler.openFileMonitor = monitor.NewLinuxFD(monitorInterval, logger)
	doppler.uptimeMonitor = monitor.NewUptime(monitorInterval)

	return doppler, nil
}

func (doppler *Doppler) Start() {
	doppler.errChan = make(chan error)

	doppler.wg.Add(8 + doppler.dropsondeUnmarshallerCollection.Size())

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
			doppler.envelopeChan <- env
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
		defer func() {
			doppler.wg.Done()
			close(doppler.envelopeChan)
		}()
		doppler.messageRouter.Start(doppler.envelopeChan)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.websocketServer.Start()
	}()

	go doppler.uptimeMonitor.Start()
	go doppler.openFileMonitor.Start()

	// The following runs forever. Put all startup functions above here.
	for err := range doppler.errChan {
		doppler.Errorf("Got error %s", err)
	}
}

func (doppler *Doppler) Stop() {
	go doppler.udpListener.Stop()
	go doppler.tcpListener.Stop()
	go doppler.tlsListener.Stop()
	go doppler.sinkManager.Stop()
	go doppler.messageRouter.Stop()
	go doppler.websocketServer.Stop()
	doppler.appStoreWatcher.Stop()
	doppler.wg.Wait()

	doppler.storeAdapter.Disconnect()
	close(doppler.errChan)
	doppler.uptimeMonitor.Stop()
	doppler.openFileMonitor.Stop()
}

func initializeMetrics(batchIntervalMilliseconds uint) *metricbatcher.MetricBatcher {
	eventEmitter := dropsonde.AutowiredEmitter()
	metricSender := metric_sender.NewMetricSender(eventEmitter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(batchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)
	return metricBatcher
}
