package loggregator

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"loggregator/iprange"
	"loggregator/sinkserver"
	"loggregator/sinkserver/unmarshaller"
	"time"
)

type Config struct {
	cfcomponent.Config
	Index                  uint
	IncomingPort           uint32
	OutgoingPort           uint32
	LogFilePath            string
	MaxRetainedLogMessages int
	WSMessageBufferSize    uint
	SharedSecret           string
	SkipCertVerify         bool
	BlackListIps           []iprange.IPRange
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
	errChan         chan error
	listener        agentlistener.AgentListener
	sinkManager     *sinkserver.SinkManager
	messageRouter   *sinkserver.MessageRouter
	u               *unmarshaller.LogMessageUnmarshaller
	websocketServer *sinkserver.WebsocketServer
}

func New(host string, config *Config, logger *gosteno.Logger) *Loggregator {
	keepAliveInterval := 30 * time.Second
	listener, incomingLogChan := agentlistener.NewAgentListener(fmt.Sprintf("%s:%d", host, config.IncomingPort), logger)
	u, messageChan := unmarshaller.NewLogMessageUnmarshaller(config.SharedSecret, incomingLogChan)
	sinkManager := sinkserver.NewSinkManager(config.MaxRetainedLogMessages, config.SkipCertVerify, config.BlackListIps, logger)
	return &Loggregator{
		errChan:         make(chan error),
		listener:        listener,
		u:               u,
		sinkManager:     sinkManager,
		messageRouter:   sinkserver.NewMessageRouter(messageChan, sinkManager, logger),
		websocketServer: sinkserver.NewWebsocketServer(fmt.Sprintf("%s:%d", host, config.OutgoingPort), sinkManager, keepAliveInterval, config.WSMessageBufferSize, logger),
	}
}

func (l *Loggregator) Start() {
	go l.listener.Start()
	go l.u.Start(l.errChan)
	go l.sinkManager.Start()
	go l.messageRouter.Start()
	go l.websocketServer.Start()
}

func (l *Loggregator) Stop() {
	l.listener.Stop()
	l.u.Stop()
	l.sinkManager.Stop()
	l.messageRouter.Stop()
	l.websocketServer.Stop()
}

func (l *Loggregator) Emitters() []instrumentation.Instrumentable {
	return []instrumentation.Instrumentable{l.listener, l.messageRouter, l.sinkManager}
}
