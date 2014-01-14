package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"path/filepath"
	"strconv"
)

type task struct {
	applicationId       string
	drainUrls           []string
	index               uint64
	wardenJobId         uint64
	wardenContainerPath string
}

func (task task) identifier() string {
	return filepath.Join(task.wardenContainerPath, "jobs", strconv.FormatUint(task.wardenJobId, 10))
}

func (task task) startListening(emitter emitter.Emitter, logger *gosteno.Logger) {
	newLoggingStream(task, emitter, logger, logmessage.LogMessage_OUT).listen()
	newLoggingStream(task, emitter, logger, logmessage.LogMessage_ERR).listen()
}
