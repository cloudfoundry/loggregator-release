package logger

import (
	"os"
	"strings"

	"github.com/cloudfoundry/gosteno"
)

func NewLogger(verbose bool, logFilePath, name string, syslogNamespace string) *gosteno.Logger {
	level := gosteno.LOG_INFO

	if verbose {
		level = gosteno.LOG_DEBUG
	}

	loggingConfig := &gosteno.Config{
		Sinks:     make([]gosteno.Sink, 1),
		Level:     level,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true}

	if strings.TrimSpace(logFilePath) == "" {
		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	} else {
		loggingConfig.Sinks[0] = gosteno.NewFileSink(logFilePath)
	}

	if syslogNamespace != "" {
		loggingConfig.Sinks = append(loggingConfig.Sinks, GetNewSyslogSink(syslogNamespace))
	}

	gosteno.Init(loggingConfig)
	logger := gosteno.NewLogger(name)
	logger.Debugf("Component %s in debug mode!", name)

	return logger
}
