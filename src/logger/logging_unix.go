// +build !windows,!plan9

package logger

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudfoundry/gosteno"
)

func GetNewSyslogSink(namespace string) *gosteno.Syslog {
	return gosteno.NewSyslogSink(namespace)
}

func RegisterGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}