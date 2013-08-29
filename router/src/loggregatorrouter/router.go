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
	"github.com/stathat/consistent"
)

type LoggregatorRouterMonitor struct {
}

func (hm LoggregatorRouterMonitor) Ok() bool {
	return true
}

type router struct {
	cfcomponent.Component
	c             *consistent.Consistent
	lcs           map[string]loggregatorclient.LoggregatorClient
	agentListener agentlistener.AgentListener
	host          string
}

func (h router) Start(logger *gosteno.Logger) {
	dataChan := h.agentListener.Start()
	for {
		dataToProxy := <-dataChan
		appId, err := appIdFromLogMessage(dataToProxy)
		if err != nil {
			logger.Warn(err.Error())
		} else {
			lc := h.lookupLoggregatorClientForAppId(appId)
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

func (h router) lookupLoggregatorClientForAppId(appId string) loggregatorclient.LoggregatorClient {
	key, _ := h.c.Get(appId)
	return h.lcs[key]
}

func NewRouter(host string, loggregatorServers []string, config cfcomponent.Config, logger *gosteno.Logger) (h *router, err error) {
	var instrumentables []instrumentation.Instrumentable
	c := consistent.New()
	c.Set(loggregatorServers)

	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(loggregatorServers))

	for _, server := range loggregatorServers {
		client := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = client
		instrumentables = append(instrumentables, client)
	}

	al := agentlistener.NewAgentListener(host, logger)
	instrumentables = append(instrumentables, al)

	cfc, err := cfcomponent.NewComponent(
		"",
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

	h = &router{Component: cfc, c: c, lcs: loggregatorClients, agentListener: al, host: host}

	return
}
