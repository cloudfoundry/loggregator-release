package main

import (
	"fmt"
	"sync"
	"time"

	"doppler/config"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
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

	errChan           chan error
	dropsondeListener agentlistener.AgentListener
	sinkManager       *sinkmanager.SinkManager
	messageRouter     *sinkserver.MessageRouter
	websocketServer   *websocketserver.WebsocketServer

	dropsondeUnmarshallerCollection dropsonde_unmarshaller.DropsondeUnmarshallerCollection
	dropsondeBytesChan              <-chan []byte
	dropsondeVerifiedBytesChan      chan []byte
	envelopeChan                    chan *events.Envelope
	wrappedEnvelopeChan             chan *events.Envelope
	signatureVerifier               *signature.Verifier

	storeAdapter storeadapter.StoreAdapter

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	sync.Mutex
	sync.WaitGroup
}

func New(host string, config *config.Config, logger *gosteno.Logger, storeAdapter storeadapter.StoreAdapter, dropsondeOrigin string) *Doppler {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second

	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)

	dropsondeListener, dropsondeBytesChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.DropsondeIncomingMessagesPort), logger, "dropsondeListener")

	signatureVerifier := signature.NewVerifier(logger, config.SharedSecret)

	unmarshallerCollection := dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(logger, config.UnmarshallerCount)

	blacklist := blacklist.New(config.BlackListIps)
	metricTTL := time.Duration(config.ContainerMetricTTLSeconds) * time.Second
	sinkTimeout := time.Duration(config.SinkInactivityTimeoutSeconds) * time.Second
	sinkManager := sinkmanager.New(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger, dropsondeOrigin, sinkTimeout, metricTTL)

	return &Doppler{
		Logger:                          logger,
		dropsondeListener:               dropsondeListener,
		sinkManager:                     sinkManager,
		messageRouter:                   sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:                 websocketserver.New(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.WSMessageBufferSize, dropsondeOrigin, logger),
		newAppServiceChan:               newAppServiceChan,
		deletedAppServiceChan:           deletedAppServiceChan,
		appStoreWatcher:                 appStoreWatcher,
		storeAdapter:                    storeAdapter,
		dropsondeBytesChan:              dropsondeBytesChan,
		dropsondeUnmarshallerCollection: unmarshallerCollection,
		envelopeChan:                    make(chan *events.Envelope),
		wrappedEnvelopeChan:             make(chan *events.Envelope),
		signatureVerifier:               signatureVerifier,
		dropsondeVerifiedBytesChan:      make(chan []byte),
	}
}

func (doppler *Doppler) Start() {
	doppler.Lock()
	doppler.errChan = make(chan error)
	doppler.Unlock()

	doppler.Add(6 + doppler.dropsondeUnmarshallerCollection.Size())

	go func() {
		defer doppler.Done()
		doppler.appStoreWatcher.Run()
	}()

	go func() {
		defer doppler.Done()
		doppler.dropsondeListener.Start()
	}()

	doppler.dropsondeUnmarshallerCollection.Run(doppler.dropsondeVerifiedBytesChan, doppler.envelopeChan, &doppler.WaitGroup)

	go func() {
		defer doppler.Done()
		defer close(doppler.dropsondeVerifiedBytesChan)
		doppler.signatureVerifier.Run(doppler.dropsondeBytesChan, doppler.dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer doppler.Done()
		doppler.sinkManager.Start(doppler.newAppServiceChan, doppler.deletedAppServiceChan)
	}()

	go func() {
		defer doppler.Done()
		defer close(doppler.envelopeChan)
		doppler.messageRouter.Start(doppler.envelopeChan)
	}()

	go func() {
		defer doppler.Done()
		doppler.websocketServer.Start()
	}()

	for err := range doppler.errChan {
		doppler.Errorf("Got error %s", err)
	}
}

func (l *Doppler) Stop() {
	l.Lock()
	defer l.Unlock()
	l.dropsondeListener.Stop()
	l.sinkManager.Stop()
	l.messageRouter.Stop()
	l.websocketServer.Stop()
	l.storeAdapter.Disconnect()

	l.Wait()
	close(l.errChan)
}
