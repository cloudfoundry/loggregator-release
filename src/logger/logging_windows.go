// +build windows

package logger

import (
	"github.com/cloudfoundry/gosteno"
)

func GetNewSyslogSink(namespace string) gosteno.Sink {
	panic("Syslog is not supported on windows")
	return nil
}
