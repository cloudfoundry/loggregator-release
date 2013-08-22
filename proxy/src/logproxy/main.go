package main

import (
	"code.google.com/p/gogoprotobuf/proto"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stathat/consistent"
	"loggregator/agentlistener"
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("v", false, "Verbose logging")
)

type hasher struct {
	c   *consistent.Consistent
	lcs map[string]loggregatorclient.LoggregatorClient
}

func main() {
	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator")

	agentListener := agentlistener.NewAgentListener(fmt.Sprintf("0.0.0.0:%d", 3456), logger)
	dataChan := agentListener.Start()

	loggregatorServers := []string{"1.2.3.4:6789", "2.3.4.5:8434"}
	// BLOW UP IF ABOVE len(loggregatorServers) == 0
	h := newHasher(loggregatorServers, logger)

	for {
		dataToProxy := <-dataChan
		if appId, err := appIdFromLogMessage(dataToProxy); err != nil {
			logger.Warn(err.Error())
		} else {
			lc := h.lookupLoggregatorClientForAppId(appId)
			go lc.Send(dataToProxy)
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

func (h hasher) lookupLoggregatorClientForAppId(appId string) loggregatorclient.LoggregatorClient {
	key, _ := h.c.Get(appId)
	return h.lcs[key]
}

func newHasher(loggregatorServers []string, logger *gosteno.Logger) (h *hasher) {
	c := consistent.New()
	c.Set(loggregatorServers)

	loggregatorClients := make(map[string]loggregatorclient.LoggregatorClient, len(loggregatorServers))

	for _, server := range loggregatorServers {
		tmpClient := loggregatorclient.NewLoggregatorClient(server, logger, loggregatorclient.DefaultBufferSize)
		loggregatorClients[server] = tmpClient
	}

	h = &hasher{c: c, lcs: loggregatorClients}

	return
}
