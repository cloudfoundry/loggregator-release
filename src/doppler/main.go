package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
)

var (
	logFilePath     = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel        = flag.Bool("debug", false, "Debug logging")
	configFile      = flag.String("config", "config/doppler.json", "Location of the doppler config json file")
	cpuprofile      = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile      = flag.String("memprofile", "", "write memory profile to this file")
	dropsondeOrigin = flag.String("dropsondeOrigin", "", "origin for messages created by this process")
)

type DopplerServerHealthMonitor struct {
}

func (hm DopplerServerHealthMonitor) Ok() bool {
	return true
}

var StoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workerPool := workerpool.NewWorkerPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workerPool)
}

const HeartbeatInterval = 10 * time.Second

func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

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

	config, logger := ParseConfig(logLevel, configFile, logFilePath)

	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to regsiter component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.ApceraWrapperNATSClient, error) {
			return fakeyagnats.NewApceraClientWrapper(), nil
		}
	}

	err := config.Validate(logger)
	if err != nil {
		panic(err)
	}

	doppler := New("0.0.0.0", config, logger, *dropsondeOrigin)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"DopplerServer",
		config.Index,
		&DopplerServerHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		doppler.Emitters(),
	)

	if err != nil {
		panic(err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(cfc)
	if err != nil {
		logger.Warnf("Unable to register with collector. Err: %v.", err)
	}

	go func() {
		err := cfc.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()

	go doppler.Start()
	logger.Info("Startup: doppler server started.")

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	StartHeartbeats(HeartbeatInterval, config, logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			doppler.Stop()
			return
		}
	}
}

func ParseConfig(logLevel *bool, configFile, logFilePath *string) (*Config, *gosteno.Logger) {
	config := &Config{}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "doppler", config.Config)
	logger.Info("Startup: Setting up the doppler server")

	return config, logger
}

func StartHeartbeats(ttl time.Duration, config *Config, logger *gosteno.Logger) (stopChan chan (chan bool)) {
	if len(config.EtcdUrls) == 0 {
		return
	}

	adapter := StoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	adapter.Connect()

	local_ip, err := localip.LocalIP()
	if err != nil {
		panic(errors.New("StartHeartbeats: unable to resolve own IP address: " + err.Error()))
	}

	logger.Debugf("Starting Health Status Updates to Store: /healthstatus/doppler/%s/%s/%d", config.Zone, config.JobName, config.Index)
	status, stopChan, err := adapter.MaintainNode(storeadapter.StoreNode{
		Key:   fmt.Sprintf("/healthstatus/doppler/%s/%s/%d", config.Zone, config.JobName, config.Index),
		Value: []byte(local_ip),
		TTL:   uint64(ttl.Seconds()),
	})

	if err != nil {
		panic(err)
	}

	go func() {
		for stat := range status {
			logger.Debugf("Health updates channel pushed %v at time %v", stat, time.Now())
		}
	}()

	return stopChan
}
