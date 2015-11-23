package monitor

import (
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type UptimeMonitor struct {
	interval time.Duration
	started  int64
	doneChan chan chan struct{}
}

func NewUptimeMonitor(interval time.Duration) Monitor {
	return &UptimeMonitor{
		interval: interval,
		started:  time.Now().Unix(),
		doneChan: make(chan chan struct{}),
	}
}

func (u *UptimeMonitor) Start() {
	ticker := time.NewTicker(u.interval)

	for {
		select {
		case <-ticker.C:
			metrics.SendValue("Uptime", float64(time.Now().Unix()-u.started), "seconds")
		case stopped := <-u.doneChan:
			ticker.Stop()
			close(stopped)
			return
		}
	}
}

func (u *UptimeMonitor) Stop() {
	stopped := make(chan struct{})
	u.doneChan <- stopped
	<-stopped
}
