package loggregatorrouter

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
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
		appId, _, err := appid.FromLogMessage(dataToProxy)
		if err != nil {
			logger.Warn(err.Error())
		} else {
			lc := r.lookupLoggregatorClientForAppId(appId)
			lc.Send(dataToProxy)
		}
	}
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
