package main

import (
	"fmt"
	"sync"
	"time"

	"doppler/config"
	"doppler/listeners/agentlistener"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"

	"common/monitor"

	"crypto/tls"
	"doppler/listeners/tlslistener"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
)

type Doppler struct {
	*gosteno.Logger
	appStoreWatcher *store.AppServiceStoreWatcher

	errChan              chan error
	dropsondeUDPListener agentlistener.Listener
	dropsondeTLSListener agentlistener.Listener
	sinkManager          *sinkmanager.SinkManager
	messageRouter        *sinkserver.MessageRouter
	websocketServer      *websocketserver.WebsocketServer

	dropsondeUnmarshallerCollection dropsonde_unmarshaller.DropsondeUnmarshallerCollection
	dropsondeBytesChan              <-chan []byte
	dropsondeVerifiedBytesChan      chan []byte
	envelopeChan                    chan *events.Envelope
	wrappedEnvelopeChan             chan *events.Envelope
	signatureVerifier               *signature.Verifier

	storeAdapter storeadapter.StoreAdapter

	uptimeMonitor monitor.Monitor

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	wg                                       sync.WaitGroup
}

func New(host string, config *config.Config, logger *gosteno.Logger, storeAdapter storeadapter.StoreAdapter, messageDrainBufferSize uint, dropsondeOrigin string, dialTimeout time.Duration) *Doppler {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second

	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)

	var dropsondeUDPListener agentlistener.Listener
	var dropsondeTLSListener agentlistener.Listener
	var dropsondeBytesChan <-chan []byte
	listenerEnvelopeChan := make(chan *events.Envelope)
	if config.EnableTLSTransport {
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{config.TLSListenerConfig.Cert},
			InsecureSkipVerify: config.TLSListenerConfig.InsecureSkipVerify,
		}
		dropsondeTLSListener = tlslistener.New(fmt.Sprintf("%s:%d", host, config.TLSListenerConfig.Port), tlsConfig, listenerEnvelopeChan, logger)
	}

	dropsondeUDPListener, dropsondeBytesChan = agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.DropsondeIncomingMessagesPort), logger, "dropsondeListener")

	signatureVerifier := signature.NewVerifier(logger, config.SharedSecret)

	unmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(logger, config.UnmarshallerCount)

	blacklist := blacklist.New(config.BlackListIps)
	metricTTL := time.Duration(config.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(config.SinkInactivityTimeoutSeconds) * time.Second
	sinkIOTimeout := time.Duration(config.SinkIOTimeoutSeconds) * time.Second
	sinkManager := sinkmanager.New(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger, messageDrainBufferSize, dropsondeOrigin, sinkTimeout, sinkIOTimeout, metricTTL, dialTimeout)

	return &Doppler{
		Logger:                          logger,
		dropsondeUDPListener:            dropsondeUDPListener,
		dropsondeTLSListener:            dropsondeTLSListener,
		sinkManager:                     sinkManager,
		messageRouter:                   sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:                 websocketserver.New(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.MessageDrainBufferSize, dropsondeOrigin, logger),
		newAppServiceChan:               newAppServiceChan,
		deletedAppServiceChan:           deletedAppServiceChan,
		appStoreWatcher:                 appStoreWatcher,
		storeAdapter:                    storeAdapter,
		dropsondeBytesChan:              dropsondeBytesChan,
		dropsondeUnmarshallerCollection: unmarshallerCollection,
		envelopeChan:                    listenerEnvelopeChan,
		wrappedEnvelopeChan:             make(chan *events.Envelope),
		signatureVerifier:               signatureVerifier,
		dropsondeVerifiedBytesChan:      make(chan []byte),
		uptimeMonitor:                   monitor.NewUptimeMonitor(time.Duration(config.MonitorIntervalSeconds) * time.Second),
	}
}

func (doppler *Doppler) Start() {
	doppler.errChan = make(chan error)

	doppler.wg.Add(6 + doppler.dropsondeUnmarshallerCollection.Size())

	go func() {
		defer doppler.wg.Done()
		doppler.appStoreWatcher.Run()
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.dropsondeUDPListener.Start()
	}()

	if doppler.dropsondeTLSListener != nil {
		doppler.wg.Add(1)
		go func() {
			defer doppler.wg.Done()
			doppler.dropsondeTLSListener.Start()
		}()
	}

	doppler.dropsondeUnmarshallerCollection.Run(doppler.dropsondeVerifiedBytesChan, doppler.envelopeChan, &doppler.wg)

	go func() {
		defer doppler.wg.Done()
		defer close(doppler.dropsondeVerifiedBytesChan)
		doppler.signatureVerifier.Run(doppler.dropsondeBytesChan, doppler.dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.sinkManager.Start(doppler.newAppServiceChan, doppler.deletedAppServiceChan)
	}()

	go func() {
		defer doppler.wg.Done()
		defer close(doppler.envelopeChan)
		doppler.messageRouter.Start(doppler.envelopeChan)
	}()

	go func() {
		defer doppler.wg.Done()
		doppler.websocketServer.Start()
	}()

	go doppler.uptimeMonitor.Start()

	// The following runs forever. Put all startup functions above here.
	for err := range doppler.errChan {
		doppler.Errorf("Got error %s", err)
	}
}

func (doppler *Doppler) Stop() {
	doppler.dropsondeUDPListener.Stop()
	doppler.dropsondeTLSListener.Stop()
	doppler.sinkManager.Stop()
	doppler.messageRouter.Stop()
	doppler.websocketServer.Stop()
	doppler.storeAdapter.Disconnect()

	doppler.wg.Wait()
	close(doppler.errChan)
	doppler.uptimeMonitor.Stop()
}
