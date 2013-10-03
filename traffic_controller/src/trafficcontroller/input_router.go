package trafficcontroller

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/appid"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"trafficcontroller/hasher"
)

type TrafficControllerMonitor struct {
}

func (hm TrafficControllerMonitor) Ok() bool {
	return true
}

type router struct {
	cfcomponent.Component
	h             *hasher.Hasher
	lcs           map[string]loggregatorclient.LoggregatorClient
	agentListener agentlistener.AgentListener
	host          string
}

func (r router) Start(logger *gosteno.Logger) {
	dataChan := r.agentListener.Start()
	for {
		dataToProxy := <-dataChan
		appId, err := appid.FromLogMessage(dataToProxy)
		if err != nil {
			logger.Warn(err.Error())
		} else {
			lc := r.lookupLoggregatorClientForAppId(appId)
			lc.Send(dataToProxy)
		}
	}
}

func (r router) lookupLoggregatorClientForAppId(appId string) loggregatorclient.LoggregatorClient {
	ls, _ := r.h.GetLoggregatorServerForAppId(appId)
	return r.lcs[ls]
}

func NewRouter(host string, h *hasher.Hasher, config cfcomponent.Config, logger *gosteno.Logger) (r *router, err error) {
	var instrumentables []instrumentation.Instrumentable
	servers := h.LoggregatorServers()
	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(servers))

	for _, server := range servers {
		client := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = client
		instrumentables = append(instrumentables, client)
	}

	al := agentlistener.NewAgentListener(host, logger)
	instrumentables = append(instrumentables, al)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"TrafficController",
		0,
		&TrafficControllerMonitor{},
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
