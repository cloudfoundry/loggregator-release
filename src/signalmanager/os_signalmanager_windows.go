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
