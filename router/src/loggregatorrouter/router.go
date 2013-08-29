package loggregatorrouter

import (
	"code.google.com/p/gogoprotobuf/proto"
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stathat/consistent"
)

type router struct {
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

func (h router) Metrics() []instrumentation.Metric {
	var metrics []instrumentation.Metric
	for _, lc := range h.lcs {
		metrics = append(metrics, lc.Metrics()...)
	}
	metrics = append(metrics, h.agentListener.Metrics()...)
	return metrics
}

func (h router) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "loggregatorRouter", Metrics: h.Metrics()}
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

func NewRouter(host string, loggregatorServers []string, logger *gosteno.Logger) (h *router) {
	c := consistent.New()
	c.Set(loggregatorServers)

	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(loggregatorServers))

	for _, server := range loggregatorServers {
		client := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = client
	}

	h = &router{c: c, lcs: loggregatorClients, agentListener: agentlistener.NewAgentListener(host, logger), host: host}

	return
}
