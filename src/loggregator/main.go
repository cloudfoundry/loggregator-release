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
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/loggregator.json", "Location of the loggregator config json file")
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile  = flag.String("memprofile", "", "write memory profile to this file")
)

type LoggregatorServerHealthMonitor struct {
}

func (hm LoggregatorServerHealthMonitor) Ok() bool {
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
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
			return fakeyagnats.New(), nil
		}
	}

	err := config.Validate(logger)
	if err != nil {
		panic(err)
	}

	l := New("0.0.0.0", config, logger)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"LoggregatorServer",
		config.Index,
		&LoggregatorServerHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		l.Emitters(),
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

	go l.Start()
	logger.Info("Startup: loggregator server started.")

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	StartHeartbeats(HeartbeatInterval, config, logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			l.Stop()
			return
		}
	}
}

func ParseConfig(logLevel *bool, configFile, logFilePath *string) (*Config, *gosteno.Logger) {
	config := &Config{IncomingPort: 3456, OutgoingPort: 8080, WSMessageBufferSize: 100}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator", config.Config)
	logger.Info("Startup: Setting up the loggregator server")

	return config, logger
}

func StartHeartbeats(ttl time.Duration, config *Config, logger *gosteno.Logger) {
	if len(config.EtcdUrls) == 0 {
		return
	}

	adapter := StoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	adapter.Connect()

	local_ip, err := localip.LocalIP()
	if err != nil {
		panic(errors.New("StartHeartbeats: unable to resolve own IP address: " + err.Error()))
	}

	logger.Debugf("Starting Health Status Updates to Store: /healthstatus/loggregator/%s/%s/%d", config.Zone, config.JobName, config.Index)
	status, _, err := adapter.MaintainNode(storeadapter.StoreNode{
		Key:   fmt.Sprintf("/healthstatus/loggregator/%s/%s/%d", config.Zone, config.JobName, config.Index),
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
}
