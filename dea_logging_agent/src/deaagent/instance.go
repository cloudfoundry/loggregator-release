package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"path/filepath"
	"strconv"
)

type instance struct {
	applicationId       string
	drainUrls           []string
	index               uint64
	wardenJobId         uint64
	wardenContainerPath string
}

func (instance instance) identifier() string {
	return filepath.Join(instance.wardenContainerPath, "jobs", strconv.FormatUint(instance.wardenJobId, 10))
}

func (inst instance) startListening(emitter emitter.Emitter, logger *gosteno.Logger) {
	newLoggingStream(inst, emitter, logger, logmessage.LogMessage_OUT).listen()
	newLoggingStream(inst, emitter, logger, logmessage.LogMessage_ERR).listen()
}
