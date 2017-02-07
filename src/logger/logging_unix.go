// +build !windows,!plan9

package logger

import (
	"github.com/cloudfoundry/gosteno"
)

func GetNewSyslogSink(namespace string) *gosteno.Syslog {
	return gosteno.NewSyslogSink(namespace)
}
