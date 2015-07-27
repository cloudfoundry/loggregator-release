package monitor

import (
	"github.com/cloudfoundry/dropsonde/metrics"
	"time"
	"sync"
)

type UptimeMonitor struct {
	interval      time.Duration
	started       int64
	doneChan	  chan struct{}
	wg            sync.WaitGroup
}

func NewUptimeMonitor(interval time.Duration) Monitor {
	return &UptimeMonitor{
		interval:      interval,
		started:       time.Now().Unix(),
		doneChan:	   make(chan struct{}),
	}
}

func (u *UptimeMonitor) Start() {
	ticker := time.NewTicker(u.interval)
	u.wg.Add(1)
	defer u.wg.Done()

	for {
		select {
		case <-ticker.C:
			metrics.SendValue("Uptime", float64(time.Now().Unix()-u.started), "seconds")
		case <-u.doneChan:
			ticker.Stop()
			return
		}
	}
}

func (u *UptimeMonitor) Stop() {
	close(u.doneChan)
	u.wg.Wait()
}
