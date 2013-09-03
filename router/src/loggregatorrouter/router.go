package loggregatorrouter

import (
	"code.google.com/p/gogoprotobuf/proto"
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type LoggregatorRouterMonitor struct {
}

func (hm LoggregatorRouterMonitor) Ok() bool {
	return true
}

type router struct {
	cfcomponent.Component
	h             *hasher
	lcs           map[string]loggregatorclient.LoggregatorClient
	agentListener agentlistener.AgentListener
	host          string
}

func (r router) Start(logger *gosteno.Logger) {
	dataChan := r.agentListener.Start()
	for {
		dataToProxy := <-dataChan
		appId, err := appIdFromLogMessage(dataToProxy)
		if err != nil {
			logger.Warn(err.Error())
		} else {
			lc := r.lookupLoggregatorClientForAppId(appId)
			lc.Send(dataToProxy)
		}
	}
}

func appIdFromLogMessage(data []byte) (appId string, err error) {
	receivedMessage := &logmessage.LogMessage{}
	err = proto.Unmarshal(data, receivedMessage)
	if err != nil {
		err = errors.New(fmt.Sprintf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data))
		return
	}
	return *receivedMessage.AppId, err
}

func (r router) lookupLoggregatorClientForAppId(appId string) loggregatorclient.LoggregatorClient {
	ls, _ := r.h.getLoggregatorServerForAppId(appId)
	return r.lcs[ls]
}

func NewRouter(host string, h *hasher, config cfcomponent.Config, logger *gosteno.Logger) (r *router, err error) {
	var instrumentables []instrumentation.Instrumentable
	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(h.loggregatorServers()))

	for _, server := range h.loggregatorServers() {
		client := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = client
		instrumentables = append(instrumentables, client)
	}

	al := agentlistener.NewAgentListener(host, logger)
	instrumentables = append(instrumentables, al)

	cfc, err := cfcomponent.NewComponent(
		0,
		"LoggregatorRouter",
		0,
		&LoggregatorRouterMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		instrumentables,
	)

	if err != nil {
		return
	}

	r = &router{Component: cfc, h: h, lcs: loggregatorClients, agentListener: al, host: host}

	return
}
