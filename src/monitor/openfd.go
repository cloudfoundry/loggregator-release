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

// TODO: exclude windows build

type OpenFileDescriptor struct {
	interval time.Duration
	done     chan chan struct{}
	logger   *gosteno.Logger
}

func NewOpenFD(interval time.Duration, logger *gosteno.Logger) *OpenFileDescriptor {
	return &OpenFileDescriptor{
		interval: interval,
		done:     make(chan chan struct{}),
		logger:   logger,
	}
}

func (o *OpenFileDescriptor) Start() {
	o.logger.Info("Starting Open File Descriptor Monitor...")

	ticker := time.NewTicker(o.interval)
	path := fmt.Sprintf("/proc/%d/fd", os.Getpid())

	for {
		select {
		case <-ticker.C:
			finfos, err := ioutil.ReadDir(path)
			if err != nil {
				o.logger.Errorf("Could not read pid dir %s: %s", path, err)
				break
			}

			metrics.SendValue("OpenFileDescriptor", float64(symlinks(finfos)), "File")
		case stopped := <-o.done:
			ticker.Stop()
			close(stopped)
			return
		}
	}
}

func (o *OpenFileDescriptor) Stop() {
	stopped := make(chan struct{})
	o.done <- stopped
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
