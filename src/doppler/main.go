package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"time"

	"doppler/config"
	"doppler/dopplerservice"

	"logger"

	"signalmanager"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

const (
	DOPPLER_ORIGIN = "DopplerServer"
	TCPTimeout     = time.Minute
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/doppler.json", "Location of the doppler config json file")
)

func NewStoreAdapter(conf *config.Config) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(conf.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.EtcdUrls,
	}
	if conf.EtcdRequireTLS {
		options.IsSSL = true
		options.CertFile = conf.EtcdTLSClientConfig.CertFile
		options.KeyFile = conf.EtcdTLSClientConfig.KeyFile
		options.CAFile = conf.EtcdTLSClientConfig.CAFile
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

	flag.Parse()

	localIp, err := localip.LocalIP()
	if err != nil {
		fatal("Unable to resolve own IP address: %s", err)
	}

	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		fatal("Unable to parse config: %s", err)
	}

	log := logger.NewLogger(*logLevel, *logFilePath, "doppler", conf.Syslog)

	go func() {
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", conf.PPROFPort), nil)
		if err != nil {
			log.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	log.Info("Startup: Setting up the doppler server")
	dropsonde.Initialize(conf.MetronAddress, DOPPLER_ORIGIN)
	storeAdapter := NewStoreAdapter(conf)

	doppler, err := New(log, localIp, conf, storeAdapter, conf.MessageDrainBufferSize, DOPPLER_ORIGIN, time.Duration(conf.WebsocketWriteTimeoutSeconds)*time.Second, time.Duration(conf.SinkDialTimeoutSeconds)*time.Second)

	if err != nil {
		fatal("Failed to create doppler: %s", err)
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

func fatal(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}
