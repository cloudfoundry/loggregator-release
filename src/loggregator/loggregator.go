package main

import (
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
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"loggregator/iprange"
	"loggregator/sinkserver"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/sinkmanager"
	"loggregator/sinkserver/unmarshaller"
	"loggregator/sinkserver/websocketserver"
	"sync"
	"time"
)

type Config struct {
	cfcomponent.Config
	EtcdUrls                         []string
	EtcdMaxConcurrentRequests        int
	Index                            uint
	LegacyIncomingMessagesPort       uint32
	DropsondeIncomingMessagesPort    uint32
	OutgoingPort                     uint32
	LogFilePath                      string
	MaxRetainedLogMessages           uint32
	WSMessageBufferSize              uint
	SharedSecret                     string
	SkipCertVerify                   bool
	BlackListIps                     []iprange.IPRange
	JobName                          string
	Zone                             string
	InactivityDurationInMilliseconds int
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

type Loggregator struct {
	*gosteno.Logger
	appStore        *store.AppServiceStore
	appStoreWatcher *store.AppServiceStoreWatcher

	appStoreInputChan <-chan appservice.AppServices

	errChan           chan error
	listener          agentlistener.AgentListener
	dropsondeListener agentlistener.AgentListener
	sinkManager       *sinkmanager.SinkManager
	messageRouter     *sinkserver.MessageRouter
	messageChan       <-chan *logmessage.Message
	unmarshaller      *unmarshaller.LogMessageUnmarshaller
	websocketServer   *websocketserver.WebsocketServer

	dropsondeUnmarshaller      dropsonde_unmarshaller.DropsondeUnmarshaller
	dropsondeBytesChan         <-chan []byte
	dropsondeVerifiedBytesChan chan []byte
	dropsondeChan              chan *events.Envelope
	signatureVerifier          signature.SignatureVerifier

	storeAdapter storeadapter.StoreAdapter

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	sync.Mutex
	sync.WaitGroup
}

func New(host string, config *Config, logger *gosteno.Logger) *Loggregator {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second
	listener, incomingLogChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.LegacyIncomingMessagesPort), logger, "agentListener")
	dropsondeListener, dropsondeBytesChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.DropsondeIncomingMessagesPort), logger, "dropsondeListener")

	unmarshaller, messageChan := unmarshaller.NewLogMessageUnmarshaller(config.SharedSecret, incomingLogChan)
	blacklist := blacklist.New(config.BlackListIps)
	sinkManager, appStoreInputChan := sinkmanager.NewSinkManager(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger, time.Duration(config.InactivityDurationInMilliseconds)*time.Millisecond)
	workerPool := workerpool.NewWorkerPool(config.EtcdMaxConcurrentRequests)

	dropsondeUnmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)
	signatureVerifier := signature.NewSignatureVerifier(logger, config.SharedSecret)

	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workerPool)
	storeAdapter.Connect()
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)
	appStore := store.NewAppServiceStore(storeAdapter, appStoreWatcher)
	return &Loggregator{
		Logger:                     logger,
		listener:                   listener,
		dropsondeListener:          dropsondeListener,
		unmarshaller:               unmarshaller,
		sinkManager:                sinkManager,
		messageChan:                messageChan,
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
		dropsondeChan:              make(chan *events.Envelope),
		signatureVerifier:          signatureVerifier,
		dropsondeVerifiedBytesChan: make(chan []byte),
	}
}

func (l *Loggregator) Start() {
	l.Lock()
	l.errChan = make(chan error)
	l.Unlock()

	err := l.storeAdapter.Connect()
	if err != nil {
		panic(err)
	}
	l.Add(11)

	go func() {
		defer l.Done()
		l.appStoreWatcher.Run()
	}()

	go func() {
		defer l.Done()
		l.appStore.Run(l.appStoreInputChan)
	}()

	go func() {
		defer l.Done()
		l.listener.Start()
	}()

	go func() {
		defer l.Done()
		l.dropsondeListener.Start()
	}()

	go func() {
		defer l.Done()
		defer close(l.dropsondeChan)
		l.dropsondeUnmarshaller.Run(l.dropsondeVerifiedBytesChan, l.dropsondeChan)
	}()

	go func() {
		defer l.Done()
		for _ = range l.dropsondeChan {
		}
	}()

	go func() {
		defer l.Done()
		defer close(l.dropsondeVerifiedBytesChan)
		l.signatureVerifier.Run(l.dropsondeBytesChan, l.dropsondeVerifiedBytesChan)
	}()

	go func() {
		defer l.Done()
		l.unmarshaller.Start(l.errChan)
	}()

	go func() {
		defer l.Done()
		l.sinkManager.Start(l.newAppServiceChan, l.deletedAppServiceChan)
	}()

	go func() {
		defer l.Done()
		l.messageRouter.Start(l.messageChan)
	}()

	go func() {
		defer l.Done()
		l.websocketServer.Start()
	}()

	for err := range l.errChan {
		l.Errorf("Got error %s", err)
	}
}

func (l *Loggregator) Stop() {
	l.Lock()
	defer l.Unlock()
	l.listener.Stop()
	l.dropsondeListener.Stop()
	l.sinkManager.Stop()
	l.messageRouter.Stop()
	l.websocketServer.Stop()
	l.storeAdapter.Disconnect()

	l.Wait()
	close(l.errChan)
}

func (l *Loggregator) Emitters() []instrumentation.Instrumentable {
	return []instrumentation.Instrumentable{
		l.listener,
		l.dropsondeListener,
		l.messageRouter,
		l.sinkManager,
		l.dropsondeUnmarshaller,
		l.signatureVerifier,
	}
}
