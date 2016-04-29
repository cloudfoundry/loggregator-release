package main

import (
	"fmt"
	"sync"
	"time"

	doppler_config "doppler/config"
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
	appStoreWatcher *store.AppServiceStoreWatcher

	errChan         chan error
	udpListener     listeners.Listener
	tcpListener     listeners.Listener
	tlsListener     listeners.Listener
	sinkManager     *sinkmanager.SinkManager
	messageRouter   *sinkserver.MessageRouter
	websocketServer *websocketserver.WebsocketServer

	dropsondeUnmarshallerCollection dropsonde_unmarshaller.DropsondeUnmarshallerCollection
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

func New(logger *gosteno.Logger,
	host string,
	config *doppler_config.Config,
	storeAdapter storeadapter.StoreAdapter,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	websocketWriteTimeout time.Duration,
	dialTimeout time.Duration) (*Doppler, error) {

	keepAliveInterval := 30 * time.Second

	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache, logger)

	var udpListener listeners.Listener
	var tlsListener listeners.Listener
	var tcpListener listeners.Listener
	var dropsondeBytesChan <-chan []byte
	var err error
	listenerEnvelopeChan := make(chan *events.Envelope)

	udpListener, dropsondeBytesChan = listeners.NewUDPListener(fmt.Sprintf("%s:%d", host, config.IncomingUDPPort), logger, "udpListener")

	var addr string
	var tlsConfig *doppler_config.TLSListenerConfig
	var contextName string

	if config.EnableTLSTransport {
		tlsConfig = &config.TLSListenerConfig
		addr = fmt.Sprintf("%s:%d", host, tlsConfig.Port)
		contextName = "tlsListener"
		tlsListener, err = listeners.NewTCPListener(contextName, addr, tlsConfig, listenerEnvelopeChan, logger)
		if err != nil {
			return nil, err
		}
	}

	tlsConfig = nil
	addr = fmt.Sprintf("%s:%d", host, config.IncomingTCPPort)
	contextName = "tcpListener"
	tcpListener, err = listeners.NewTCPListener(contextName, addr, tlsConfig, listenerEnvelopeChan, logger)

	signatureVerifier := signature.NewVerifier(logger, config.SharedSecret)

	unmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(logger, config.UnmarshallerCount)

	blacklist := blacklist.New(config.BlackListIps, logger)
	metricTTL := time.Duration(config.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(config.SinkInactivityTimeoutSeconds) * time.Second
	sinkIOTimeout := time.Duration(config.SinkIOTimeoutSeconds) * time.Second
	sinkManager := sinkmanager.New(config.MaxRetainedLogMessages, config.SinkSkipCertVerify, blacklist, logger, messageDrainBufferSize, dropsondeOrigin, sinkTimeout, sinkIOTimeout, metricTTL, dialTimeout)

	websocketServer, err := websocketserver.New(fmt.Sprintf(":%d", config.OutgoingPort), sinkManager, websocketWriteTimeout, keepAliveInterval, config.MessageDrainBufferSize, dropsondeOrigin, logger)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the websocket server: %s", err.Error())
	}

	initializeMetrics(config.MetricBatchIntervalMilliseconds)
	monitorInterval := time.Duration(config.MonitorIntervalSeconds) * time.Second
	openFileMonitor := monitor.NewLinuxFD(monitorInterval, logger)

	return &Doppler{
		Logger:                          logger,
		udpListener:                     udpListener,
		tcpListener:                     tcpListener,
		tlsListener:                     tlsListener,
		sinkManager:                     sinkManager,
		messageRouter:                   sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:                 websocketServer,
		newAppServiceChan:               newAppServiceChan,
		deletedAppServiceChan:           deletedAppServiceChan,
		appStoreWatcher:                 appStoreWatcher,
		storeAdapter:                    storeAdapter,
		dropsondeBytesChan:              dropsondeBytesChan,
		dropsondeUnmarshallerCollection: unmarshallerCollection,
		envelopeChan:                    listenerEnvelopeChan,
		signatureVerifier:               signatureVerifier,
		dropsondeVerifiedBytesChan:      make(chan []byte),
		uptimeMonitor:                   monitor.NewUptime(monitorInterval),
		openFileMonitor:                 openFileMonitor,
	}, nil
}

func (doppler *Doppler) Start() {
	doppler.errChan = make(chan error)

	doppler.wg.Add(7 + doppler.dropsondeUnmarshallerCollection.Size())

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
		go func() {
			doppler.wg.Add(1)
			defer doppler.wg.Done()
			doppler.tlsListener.Start()
		}()
	}

	doppler.dropsondeUnmarshallerCollection.Run(doppler.dropsondeVerifiedBytesChan, doppler.envelopeChan, &doppler.wg)

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

func initializeMetrics(batchIntervalMilliseconds uint) {
	eventEmitter := dropsonde.AutowiredEmitter()
	metricSender := metric_sender.NewMetricSender(eventEmitter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(batchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)
}
