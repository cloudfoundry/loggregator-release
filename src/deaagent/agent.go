package deaagent

import (
	"deaagent/metadataservice"
	"deaagent/syslogreader"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/emitter"
)

type Agent struct {
	listener        agentlistener.AgentListener
	metadataService metadataservice.MetaDataService
	emitter         emitter.Emitter
	logger          *gosteno.Logger
}

func NewAgent(listener agentlistener.AgentListener, metadataService metadataservice.MetaDataService, emitter emitter.Emitter, logger *gosteno.Logger) Agent {
	return Agent{listener, metadataService, emitter, logger}
}

func (agent *Agent) Start() {
	incomingBytes := agent.listener.Start()
	go func() {
		for datagram := range incomingBytes {
			logMessage, err := syslogreader.ReadMessage(agent.metadataService, datagram)
			if err != nil {
				agent.logger.Warnf("Failed generating LogMessage: %s", err)
			}
			agent.emitter.EmitLogMessage(logMessage)
		}
	}()
}
