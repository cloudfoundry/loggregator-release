package main

import (
	"doppler/envelopewrapper"
	"doppler/iprange"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"sync"
	"time"
)

type Config struct {
	cfcomponent.Config
	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	Index                         uint
	DropsondeIncomingMessagesPort uint32
	OutgoingPort                  uint32
	LogFilePath                   string
	MaxRetainedLogMessages        uint32
	WSMessageBufferSize           uint
	SharedSecret                  string
	SkipCertVerify                bool
	BlackListIps                  []iprange.IPRange
	JobName                       string
	Zone                          string
}

func (c *Config) Validate(logger *gosteno.Logger) (err error) {
	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	if c.BlackListIps != nil {
		err = iprange.ValidateIpAddresses(c.BlackListIps)
		if err != nil {
			return err
		}
	}

	err = c.Config.Validate(logger)
	return
}

type Doppler struct {
	*gosteno.Logger
	appStore        *store.AppServiceStore
	appStoreWatcher *store.AppServiceStoreWatcher

	appStoreInputChan <-chan appservice.AppServices

	errChan           chan error
	dropsondeListener agentlistener.AgentListener
	sinkManager       *sinkmanager.SinkManager
	messageRouter     *sinkserver.MessageRouter
	websocketServer   *websocketserver.WebsocketServer

	dropsondeUnmarshaller      dropsonde_unmarshaller.DropsondeUnmarshaller
	dropsondeEnvelopeWrapper   envelopewrapper.EnvelopeWrapper
	dropsondeBytesChan         <-chan []byte
	dropsondeVerifiedBytesChan chan []byte
	envelopeChan               chan *events.Envelope
	wrappedEnvelopeChan        chan *envelopewrapper.WrappedEnvelope
	signatureVerifier          signature.SignatureVerifier

	storeAdapter storeadapter.StoreAdapter

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	sync.Mutex
	sync.WaitGroup
}

func New(host string, config *Config, logger *gosteno.Logger) *Doppler {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second

	workerPool := workerpool.NewWorkerPool(config.EtcdMaxConcurrentRequests)
	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workerPool)
	storeAdapter.Connect()
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)
	appStore := store.NewAppServiceStore(storeAdapter, appStoreWatcher)

	dropsondeListener, dropsondeBytesChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.DropsondeIncomingMessagesPort), logger, "dropsondeListener")

	signatureVerifier := signature.NewSignatureVerifier(logger, config.SharedSecret)
	dropsondeUnmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)

	dropsondeEnvelopeWrapper := envelopewrapper.NewEnvelopeWrapper(logger)

	blacklist := blacklist.New(config.BlackListIps)
	sinkManager, appStoreInputChan := sinkmanager.NewSinkManager(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger)

	return &Doppler{
		Logger:                     logger,
		dropsondeListener:          dropsondeListener,
		sinkManager:                sinkManager,
		appStoreInputChan:          appStoreInputChan,
		appStore:                   appStore,
		messageRouter:              sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:            websocketserver.New(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.WSMessageBufferSize, logger),
		newAppServiceChan:          newAppServiceChan,
		deletedAppServiceChan:      deletedAppServiceChan,
		appStoreWatcher:            appStoreWatcher,
		storeAdapter:               storeAdapter,
		dropsondeBytesChan:         dropsondeBytesChan,
		dropsondeUnmarshaller:      dropsondeUnmarshaller,
		dropsondeEnvelopeWrapper:   dropsondeEnvelopeWrapper,
		envelopeChan:               make(chan *events.Envelope),
		wrappedEnvelopeChan:        make(chan *envelopewrapper.WrappedEnvelope),
		signatureVerifier:          signatureVerifier,
		dropsondeVerifiedBytesChan: make(chan []byte),
	}
}

func (d *Doppler) Start() {
	d.Lock()
	d.errChan = make(chan error)
	d.Unlock()

	err := d.storeAdapter.Connect()
	if err != nil {
		panic(err)
	}
	d.Add(9)

	go func() {
		defer d.Done()
		d.appStoreWatcher.Run()
	}()

	go func() {
		defer d.Done()
		d.appStore.Run(d.appStoreInputChan)
	}()

	go func() {
		defer d.Done()
		d.dropsondeListener.Start()
	}()

	go func() {
		defer d.Done()
		defer close(d.envelopeChan)
		d.dropsondeUnmarshaller.Run(d.dropsondeVerifiedBytesChan, d.envelopeChan)
	}()

	go func() {
		defer d.Done()
		defer close(d.wrappedEnvelopeChan)
		d.dropsondeEnvelopeWrapper.Run(d.envelopeChan, d.wrappedEnvelopeChan)
	}()

	go func() {
		defer d.Done()
		defer close(d.dropsondeVerifiedBytesChan)
		d.signatureVerifier.Run(d.dropsondeBytesChan, d.dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer d.Done()
		d.sinkManager.Start(d.newAppServiceChan, d.deletedAppServiceChan)
	}()

	go func() {
		defer d.Done()
		d.messageRouter.Start(d.wrappedEnvelopeChan)
	}()

	go func() {
		defer d.Done()
		d.websocketServer.Start()
	}()

	for err := range d.errChan {
		d.Errorf("Got error %s", err)
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

func (l *Doppler) Emitters() []instrumentation.Instrumentable {
	return []instrumentation.Instrumentable{
		l.dropsondeListener,
		l.messageRouter,
		l.sinkManager,
		l.dropsondeUnmarshaller,
		l.signatureVerifier,
	}
}
