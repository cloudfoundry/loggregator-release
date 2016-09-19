package main

import (
	"flag"
	"logger"
	"os"
	"os/signal"
	"syscall"
	"syslog_drain_binder/config"
	"time"

	"syslog_drain_binder/elector"
	"syslog_drain_binder/etcd_syslog_drain_store"

	"signalmanager"

	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	debug       = flag.Bool("debug", false, "Verbose (debug) logging")
	configFile  = flag.String("config", "config/syslog_drain_binder.json", "Location of the Syslog Drain Binder config json file")
)

func main() {
	flag.Parse()
	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		panic(err)
	}
	log := logger.NewLogger(*debug, *logFilePath, "syslog_drain_binder", conf.Syslog)

	dropsonde.Initialize(conf.MetronAddress, "syslog_drain_binder")

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
	adapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}

	updateInterval := time.Duration(conf.UpdateIntervalSeconds) * time.Second
	politician := elector.NewElector(conf.InstanceName, adapter, updateInterval, log)

	drainTTL := time.Duration(conf.DrainUrlTtlSeconds) * time.Second
	store := etcd_syslog_drain_store.NewEtcdSyslogDrainStore(adapter, drainTTL, log)

	dumpChan := registerGoRoutineDumpSignalChannel()
	ticker := time.NewTicker(updateInterval)
	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-ticker.C:
			if politician.IsLeader() {
				err = politician.StayAsLeader()
				if err != nil {
					log.Errorf("Error when staying leader: %s", err.Error())
					politician.Vacate()
					continue
				}
			} else {
				err = politician.RunForElection()

				if err != nil {
					log.Errorf("Error when running for leader: %s", err.Error())
					politician.Vacate()
					continue
				}
			}

			log.Debugf("Polling %s for updates", conf.CloudControllerAddress)
			drainUrls, err := Poll(conf.CloudControllerAddress, conf.BulkApiUsername, conf.BulkApiPassword, conf.PollingBatchSize, conf.SkipCertVerify)
			if err != nil {
				log.Errorf("Error when polling cloud controller: %s", err.Error())
				politician.Vacate()
				continue
			}

			metrics.IncrementCounter("pollCount")

			var totalDrains int
			for _, drainList := range drainUrls {
				totalDrains += len(drainList)
			}

			metrics.SendValue("totalDrains", float64(totalDrains), "drains")

			log.Debugf("Updating drain URLs for %d application(s)", len(drainUrls))
			err = store.UpdateDrains(drainUrls)
			if err != nil {
				log.Errorf("Error when updating ETCD: %s", err.Error())
				politician.Vacate()
				continue
			}
		}
	}
}

func registerGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}
