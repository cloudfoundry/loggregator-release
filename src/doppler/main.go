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

	"doppler/config"

	"crypto/tls"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/pivotal-golang/localip"
)

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

	conf, logger := ParseConfig(logLevel, configFile, logFilePath)

	dropsonde.Initialize(conf.MetronAddress, "DopplerServer")

	if len(conf.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to regsiter component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
		}
	}

	err = conf.Validate(logger)
	if err != nil {
		panic(err)
	}

	storeAdapter := NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)
	doppler := New(localIp, conf, logger, storeAdapter, conf.MessageDrainBufferSize, "doppler", time.Duration(conf.SinkDialTimeoutSeconds)*time.Second)

	if err != nil {
		panic(err)
	}

	go doppler.Start()
	logger.Info("Startup: doppler server started.")

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	storeAdapter = NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)
	StartHeartbeats(localIp, config.HeartbeatInterval, conf, storeAdapter, logger)

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

func ParseConfig(logLevel *bool, configFile, logFilePath *string) (*config.Config, *gosteno.Logger) {
	config := &config.Config{}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	if config.MonitorIntervalSeconds == 0 {
		config.MonitorIntervalSeconds = 60
	}

	if config.SinkDialTimeoutSeconds == 0 {
		config.SinkDialTimeoutSeconds = 1
	}

	if config.EnableTLSTransport {
		cert, err := tls.LoadX509KeyPair(config.TLSListenerConfig.CrtFile, config.TLSListenerConfig.KeyFile)
		if err != nil {
			panic(err)
		}
		config.TLSListenerConfig.Cert = cert
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "doppler", config.Config)
	logger.Info("Startup: Setting up the doppler server")

	return config, logger
}

func StartHeartbeats(localIp string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) (stopChan chan (chan bool)) {
	if len(config.EtcdUrls) == 0 {
		return
	}

	if storeAdapter == nil {
		panic("store adapter is nil")
	}

	logger.Debugf("Starting Health Status Updates to Store: /healthstatus/doppler/%s/%s/%d", config.Zone, config.JobName, config.Index)
	status, stopChan, err := storeAdapter.MaintainNode(storeadapter.StoreNode{
		Key:   fmt.Sprintf("/healthstatus/doppler/%s/%s/%d", config.Zone, config.JobName, config.Index),
		Value: []byte(localIp),
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
