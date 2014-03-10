package loggregator

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"loggregator/domain"
	"loggregator/iprange"
	"loggregator/sinkserver"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/sinkmanager"
	"loggregator/sinkserver/unmarshaller"
	"loggregator/sinkserver/websocket"
	"loggregator/store"
	"loggregator/store/cache"
	"time"
)

type Config struct {
	cfcomponent.Config
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int
	Index                     uint
	IncomingPort              uint32
	OutgoingPort              uint32
	LogFilePath               string
	MaxRetainedLogMessages    uint32
	WSMessageBufferSize       uint
	SharedSecret              string
	SkipCertVerify            bool
	BlackListIps              []iprange.IPRange
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

	appStoreInputChan <-chan domain.AppServices

	errChan         chan error
	listener        agentlistener.AgentListener
	sinkManager     *sinkmanager.SinkManager
	messageRouter   *sinkserver.MessageRouter
	messageChan     <-chan *logmessage.Message
	unmarshaller    *unmarshaller.LogMessageUnmarshaller
	websocketServer *websocket.WebsocketServer

	storeAdapter storeadapter.StoreAdapter

	newAppServiceChan, deletedAppServiceChan <-chan domain.AppService
}

func New(host string, config *Config, logger *gosteno.Logger) *Loggregator {
	cfcomponent.Logger = logger
	keepAliveInterval := 30 * time.Second
	listener, incomingLogChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.IncomingPort), logger)
	unmarshaller, messageChan := unmarshaller.NewLogMessageUnmarshaller(config.SharedSecret, incomingLogChan)
	blacklist := blacklist.New(config.BlackListIps)
	sinkManager, appStoreInputChan := sinkmanager.NewSinkManager(config.MaxRetainedLogMessages, config.SkipCertVerify, blacklist, logger)
	workerPool := workerpool.NewWorkerPool(config.EtcdMaxConcurrentRequests)

	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workerPool)
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, newAppServiceChan, deletedAppServiceChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)
	appStore := store.NewAppServiceStore(storeAdapter, appStoreWatcher)
	return &Loggregator{
		Logger:                logger,
		errChan:               make(chan error),
		listener:              listener,
		unmarshaller:          unmarshaller,
		sinkManager:           sinkManager,
		messageChan:           messageChan,
		appStoreInputChan:     appStoreInputChan,
		appStore:              appStore,
		messageRouter:         sinkserver.NewMessageRouter(sinkManager, logger),
		websocketServer:       websocket.NewWebsocketServer(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.WSMessageBufferSize, logger),
		newAppServiceChan:     newAppServiceChan,
		deletedAppServiceChan: deletedAppServiceChan,
		appStoreWatcher:       appStoreWatcher,
		storeAdapter:          storeAdapter,
	}
}

func (l *Loggregator) Start() {
	err := l.storeAdapter.Connect()
	if err != nil {
		panic(err)
	}

	go l.appStoreWatcher.Run()

	go l.appStore.Run(l.appStoreInputChan)
	go l.listener.Start()
	go l.unmarshaller.Start(l.errChan)
	go l.sinkManager.Start(l.newAppServiceChan, l.deletedAppServiceChan)

	go l.messageRouter.Start(l.messageChan)

	go l.websocketServer.Start()

	go func() {
		for err := range l.errChan {
			l.Errorf("Got error %s", err)
		}
	}()
}

func (l *Loggregator) Stop() {
	l.listener.Stop()
	l.unmarshaller.Stop()
	l.sinkManager.Stop()
	l.messageRouter.Stop()
	l.websocketServer.Stop()
	close(l.errChan)
}

func (l *Loggregator) Emitters() []instrumentation.Instrumentable {
	return []instrumentation.Instrumentable{l.listener, l.messageRouter, l.sinkManager}
}
