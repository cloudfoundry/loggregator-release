// +build linux

package monitor

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type LinuxFileDescriptor struct {
	interval time.Duration
	done     chan chan struct{}
	logger   *gosteno.Logger
}

func NewLinuxFD(interval time.Duration, logger *gosteno.Logger) *LinuxFileDescriptor {
	return &LinuxFileDescriptor{
		interval: interval,
		done:     make(chan chan struct{}),
		logger:   logger,
	}
}

func (l *LinuxFileDescriptor) Start() {
	l.logger.Info("Starting Open File Descriptor Monitor...")

	ticker := time.NewTicker(l.interval)
	path := fmt.Sprintf("/proc/%d/fd", os.Getpid())

	for {
		select {
		case <-ticker.C:
			finfos, err := ioutil.ReadDir(path)
			if err != nil {
				l.logger.Errorf("Could not read pid dir %s: %s", path, err)
				break
			}

			metrics.SendValue("LinuxFileDescriptor", float64(symlinks(finfos)), "File")
		case stopped := <-l.done:
			ticker.Stop()
			close(stopped)
			return
		}
	}
}

func (l *LinuxFileDescriptor) Stop() {
	stopped := make(chan struct{})
	l.done <- stopped
	<-stopped
}

func symlinks(finfos []os.FileInfo) int {
	count := 0
	for i := 0; i < len(finfos); i++ {
		if finfos[i].Mode()&os.ModeSymlink != os.ModeSymlink {
			continue
		}
		count++
	}
	return count
}
