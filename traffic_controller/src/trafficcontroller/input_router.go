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

type Router struct {
	cfcomponent.Component
	hasher             *hasher.Hasher
	loggregatorClients map[string]loggregatorclient.LoggregatorClient
	agentListener      agentlistener.AgentListener
	host               string
}

func NewRouter(host string, hasher *hasher.Hasher, config cfcomponent.Config, logger *gosteno.Logger) (r *Router, err error) {
	var instrumentables []instrumentation.Instrumentable
	servers := hasher.LoggregatorServers()
	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(servers))

	for _, server := range servers {
		client := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = client
		instrumentables = append(instrumentables, client)
	}

	agentListener := agentlistener.NewAgentListener(host, logger)
	instrumentables = append(instrumentables, agentListener)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"LoggregatorTrafficcontroller",
		0,
		&TrafficControllerMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		instrumentables,
	)

	if err != nil {
		return
	}

	r = &Router{Component: cfc, hasher: hasher, loggregatorClients: loggregatorClients, agentListener: agentListener, host: host}

	return
}

func (r Router) Start(logger *gosteno.Logger) {
	dataChan := r.agentListener.Start()
	for {
		dataToProxy := <-dataChan
		appId, err := appid.FromLogMessage(dataToProxy)
		if err != nil {
			logger.Warn(err.Error())
		} else {
			server := r.hasher.GetLoggregatorServerForAppId(appId)
			lc := r.loggregatorClients[server]
			r.Component.Logger.Debugf("Incoming Router: AppId is %v. Using server: %v", appId, server)
			lc.Send(dataToProxy)
		}
	}
}
