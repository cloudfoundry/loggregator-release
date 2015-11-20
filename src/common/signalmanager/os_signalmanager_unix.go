// +build !windows,!plan9

package signalmanager

import (
	"syscall"
	"os/signal"
	"os"
	"runtime/pprof"
)

func RegisterKillSignalChannel() chan os.Signal {
	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	return killChan
}

func RegisterGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}

func DumpGoRoutine() {
	goRoutineProfiles := pprof.Lookup("goroutine")
	goRoutineProfiles.WriteTo(os.Stdout, 2)
}

