// +build windows

package signalmanager

import (
	"os"
	"os/signal"
)

func RegisterKillSignalChannel() chan os.Signal {
	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	return killChan
}

func RegisterGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	return threadDumpChan
}

func DumpGoRoutine() {
	// no op
}
