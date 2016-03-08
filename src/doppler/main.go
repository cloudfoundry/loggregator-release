package main

import (
	"errors"
	"flag"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"doppler/config"
	"doppler/dopplerservice"

	"logger"

	"signalmanager"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/localip"
)

const (
	DOPPLER_ORIGIN = "DopplerServer"
	pprofPort      = "6060"
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/doppler.json", "Location of the doppler config json file")
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
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}
	if err = etcdStoreAdapter.Connect(); err != nil {
		panic(err)
	}
	return etcdStoreAdapter
}

func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	// Put os.Exit in a deferred statement so that other defers get executed prior to
	// the os.Exit call.
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	localIp, err := localip.LocalIP()
	if err != nil {
		panic(errors.New("Unable to resolve own IP address: " + err.Error()))
	}

	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		panic(err)
	}

	log := logger.NewLogger(*logLevel, *logFilePath, "doppler", conf.Syslog)

	go func() {
		err := http.ListenAndServe(net.JoinHostPort(localIp, pprofPort), nil)
		if err != nil {
			log.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	log.Info("Startup: Setting up the doppler server")
	dropsonde.Initialize(conf.MetronAddress, DOPPLER_ORIGIN)
	storeAdapter := NewStoreAdapter(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)

	doppler, err := New(log, localIp, conf, storeAdapter, conf.MessageDrainBufferSize, DOPPLER_ORIGIN, time.Duration(conf.WebsocketWriteTimeoutSeconds)*time.Second, time.Duration(conf.SinkDialTimeoutSeconds)*time.Second)

	if err != nil {
		log.Errorf("Failed to create doppler: %s", err.Error())
		exitCode = -1
		return
	}

	go doppler.Start()
	log.Info("Startup: doppler server started.")

	killChan := signalmanager.RegisterKillSignalChannel()
	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()

	releaseNodeChan := dopplerservice.Announce(localIp, config.HeartbeatInterval, conf, storeAdapter, log)
	legacyReleaseNodeChan := dopplerservice.AnnounceLegacy(localIp, config.HeartbeatInterval, conf, storeAdapter, log)

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Info("Shutting down")

			stopped := make(chan bool)
			legacyStopped := make(chan bool)
			releaseNodeChan <- stopped
			legacyReleaseNodeChan <- legacyStopped

			doppler.Stop()

			<-stopped
			<-legacyStopped

			return
		}
	}
}
