package monitor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

func NewOpenFileDescriptor(interval time.Duration, logger *gosteno.Logger) Monitor {
	return &OpenFileDescriptor{
		interval: interval,
		done:     make(chan chan struct{}),
		logger:   logger,
	}
}

func (ofd *OpenFileDescriptor) Start() {
	ofd.logger.Info("Starting Open File Descriptor Monitor...")

	var err error
	ticker := time.NewTicker(ofd.interval)
	path := fmt.Sprintf("/proc/%s/fd", os.Getpid())
	findCmd := exec.Command("find", path)
	wcCmd := exec.Command("wc", "-l")

	r, w := io.Pipe()
	findCmd.Stdout = w
	wcCmd.Stdin = r

	var buf bytes.Buffer
	wcCmd.Stdout = &buf
	for {
		select {
		case <-ticker.C:
			// run the command and send metric
			err = findCmd.Start()
			if err != nil {
				ofd.logger.Errorf("Unable to start find command: %s", err)
			}
			err = wcCmd.Start()
			if err != nil {
				ofd.logger.Errorf("Unable to start wc command: %s", err)
			}
			err = findCmd.Wait()
			if err != nil {
				ofd.logger.Errorf("Failed to run find command: %s", err)
			}
			w.Close()
			wcCmd.Wait()
			if err != nil {
				ofd.logger.Errorf("Failed to run wc command: %s", err)
			}

			openFiles, err := strconv.Atoi(strings.TrimSpace(buf.String()))
			if err != nil {
				ofd.logger.Errorf("Error parsing monitor output: %s", err)
			}
			metrics.SendValue("OpenFileDescriptor", float64(openFiles), "File")

		case stopped := <-ofd.done:
			ticker.Stop()
			close(stopped)
			return
		}
	}
}

func (ofd *OpenFileDescriptor) Stop() {
	stopped := make(chan struct{})
	ofd.done <- stopped
	<-stopped
}
