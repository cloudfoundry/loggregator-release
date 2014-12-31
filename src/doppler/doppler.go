package main

import (
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
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
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
	ContainerMetricTTLSeconds     int
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
	appStoreWatcher *store.AppServiceStoreWatcher

	errChan           chan error
	dropsondeListener agentlistener.AgentListener
	sinkManager       *sinkmanager.SinkManager
	messageRouter     *sinkserver.MessageRouter
	websocketServer   *websocketserver.WebsocketServer

	dropsondeUnmarshaller      dropsonde_unmarshaller.DropsondeUnmarshaller
	dropsondeBytesChan         <-chan []byte
	dropsondeVerifiedBytesChan chan []byte
	envelopeChan               chan *events.Envelope
	wrappedEnvelopeChan        chan *events.Envelope
	signatureVerifier          signature.SignatureVerifier

	storeAdapter storeadapter.StoreAdapter

	newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService
	sync.Mutex
	sync.WaitGroup
}

func New(host string, config *Config, logger *gosteno.Logger, dropsondeOrigin string) *Doppler {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second

	workPool := workpool.NewWorkPool(config.EtcdMaxConcurrentRequests)
	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workPool)
	storeAdapter.Connect()
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)

	dropsondeListener, dropsondeBytesChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.DropsondeIncomingMessagesPort), logger, "dropsondeListener")

	signatureVerifier := signature.NewSignatureVerifier(logger, config.SharedSecret)
	dropsondeUnmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)

	blacklist := blacklist.New(config.BlackListIps)
	metricTTL := time.Duration(config.ContainerMetricTTLSeconds) * time.Second
	sinkManager := sinkmanager.NewSinkManager(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger, dropsondeOrigin, metricTTL)

	return &Doppler{
		Logger:                     logger,
		dropsondeListener:          dropsondeListener,
		sinkManager:                sinkManager,
		messageRouter:              sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:            websocketserver.New(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.WSMessageBufferSize, logger),
		newAppServiceChan:          newAppServiceChan,
		deletedAppServiceChan:      deletedAppServiceChan,
		appStoreWatcher:            appStoreWatcher,
		storeAdapter:               storeAdapter,
		dropsondeBytesChan:         dropsondeBytesChan,
		dropsondeUnmarshaller:      dropsondeUnmarshaller,
		envelopeChan:               make(chan *events.Envelope),
		wrappedEnvelopeChan:        make(chan *events.Envelope),
		signatureVerifier:          signatureVerifier,
		dropsondeVerifiedBytesChan: make(chan []byte),
	}
}

func (doppler *Doppler) Start() {
	doppler.Lock()
	doppler.errChan = make(chan error)
	doppler.Unlock()

	err := doppler.storeAdapter.Connect()
	if err != nil {
		panic(err)
	}
	doppler.Add(7)

	go func() {
		defer doppler.Done()
		doppler.appStoreWatcher.Run()
	}()

	go func() {
		defer doppler.Done()
		doppler.dropsondeListener.Start()
	}()

	go func() {
		defer doppler.Done()
		defer close(doppler.envelopeChan)
		doppler.dropsondeUnmarshaller.Run(doppler.dropsondeVerifiedBytesChan, doppler.envelopeChan)
	}()

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

func (l *Doppler) Emitters() []instrumentation.Instrumentable {
	return []instrumentation.Instrumentable{
		l.dropsondeListener,
		l.messageRouter,
		l.sinkManager,
		l.dropsondeUnmarshaller,
		l.signatureVerifier,
	}
}
