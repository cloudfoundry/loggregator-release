// +build windows

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
	return threadDumpChan
}

func DumpGoRoutine() {
	// no op
}

