package profiler

import (
	"github.com/cloudfoundry/gosteno"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

type Profiler struct {
	cpuProfile       string
	memProfile       string
	cpuProfileHandle *os.File
	memProfileHandle *os.File
	ticker           *time.Ticker
	interval         time.Duration
	logger           *gosteno.Logger
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

func New(cpuProfile, memProfile string, interval time.Duration, logger *gosteno.Logger) *Profiler {
	return &Profiler{
		cpuProfile: cpuProfile,
		memProfile: memProfile,
		interval:   interval,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
}

func (p *Profiler) Profile() {
	if p.cpuProfile != "" {
		f, err := os.Create(p.cpuProfile)
		if err != nil {
			panic(err)
		}
		p.cpuProfileHandle = f
		pprof.StartCPUProfile(f)
	}

	if p.memProfile != "" {
		f, err := os.Create(p.memProfile)
		if err != nil {
			panic(err)
		}
		p.memProfileHandle = f

		ticker := time.NewTicker(p.interval)
		p.ticker = ticker

		p.wg.Add(1)
		go func() {
			p.runMemProfiler(ticker)
			p.wg.Done()
		}()

	}
}

func (p *Profiler) Stop() {
	pprof.StopCPUProfile()

	if p.ticker != nil {
		p.ticker.Stop()
		close(p.stopChan)

		p.wg.Wait()
	}

	p.cpuProfileHandle.Close()
	p.memProfileHandle.Close()
}

func (p *Profiler) runMemProfiler(ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			err := pprof.WriteHeapProfile(p.memProfileHandle)
			if err != nil {
				p.logger.Errorf("Error in profiler: %s\n", err.Error())
			}
		case <-p.stopChan:
			return
		}
	}
}

func (p *Profiler) GetCpuProfileHandle() *os.File {
	return p.cpuProfileHandle
}

func (p *Profiler) GetMemProfileHandle() *os.File {
	return p.memProfileHandle
}
