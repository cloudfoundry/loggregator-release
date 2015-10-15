package main

import (
	"errors"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"time"

	"doppler/config"

	"doppler/announcer"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"
)

const DOPPLER_ORIGIN = "doppler"

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/doppler.json", "Location of the doppler config json file")
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile  = flag.String("memprofile", "", "write memory profile to this file")
)

type DopplerServerHealthMonitor struct {
}

func (hm DopplerServerHealthMonitor) Ok() bool {
	return true
}

func NewStoreAdapter(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}
	etcdStoreAdapter := etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
	etcdStoreAdapter.Connect()
	return etcdStoreAdapter
}

func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	localIp, err := localip.LocalIP()
	if err != nil {
		panic(errors.New("Unable to resolve own IP address: " + err.Error()))
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(err)
		}
		go func() {
			defer f.Close()
			ticker := time.NewTicker(time.Second * 1)
			defer ticker.Stop()
			for {
				<-ticker.C
				pprof.WriteHeapProfile(f)
			}
		}()
	}

	conf, err := config.ParseConfig(configFile)
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "doppler", conf.Config)
	logger.Info("Startup: Setting up the doppler server")

	dropsonde.Initialize(conf.MetronAddress, DOPPLER_ORIGIN)
	dopplerStoreAdapter := NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)
	legacyStoreAdapter := NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)

	doppler := New(localIp, conf, logger, dopplerStoreAdapter, conf.MessageDrainBufferSize, DOPPLER_ORIGIN, time.Duration(conf.SinkDialTimeoutSeconds)*time.Second)

	if err != nil {
		panic(err)
	}

	go doppler.Start()
	logger.Info("Startup: doppler server started.")

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	releaseNodeChan := announcer.Announce(localIp, config.HeartbeatInterval, conf, dopplerStoreAdapter, logger)
	legacyReleaseNodeChan := announcer.AnnounceLegacy(localIp, config.HeartbeatInterval, conf, legacyStoreAdapter, logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			doppler.Stop()
			close(releaseNodeChan)
			close(legacyReleaseNodeChan)
			return
		}
	}
}
