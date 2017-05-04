// +build !windows,!plan9

package signalmanager

import (
	"os"
	"os/signal"
)

func RegisterKillSignalChannel() chan os.Signal {
	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)

	return killChan
}
