package main

import (
	"flag"
	"log"
	"plumbing"
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
	configFile  = flag.String("config", "config/syslog_drain_binder.json", "Location of the Syslog Drain Binder config json file")
)

func main() {
	flag.Parse()
	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		panic(err)
	}

	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.CloudControllerTLSConfig.CertFile,
		conf.CloudControllerTLSConfig.KeyFile,
		conf.CloudControllerTLSConfig.CAFile,
		"cloud-controller-ng.service.cf.internal",
	)
	if err != nil {
		panic(err)
	}
	tlsConfig.InsecureSkipVerify = conf.SkipCertVerify

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
	politician := elector.NewElector(conf.InstanceName, adapter, updateInterval)

	drainTTL := time.Duration(conf.DrainUrlTtlSeconds) * time.Second
	store := etcd_syslog_drain_store.NewEtcdSyslogDrainStore(adapter, drainTTL)

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
					log.Printf("Error when staying leader: %s", err.Error())
					politician.Vacate()
					continue
				}
			} else {
				err = politician.RunForElection()

				if err != nil {
					log.Printf("Error when running for leader: %s", err.Error())
					politician.Vacate()
					continue
				}
			}

			drainBindings, err := Poll(
				conf.CloudControllerAddress,
				conf.PollingBatchSize,
				tlsConfig,
			)
			if err != nil {
				log.Printf("Error when polling cloud controller: %s", err.Error())
				politician.Vacate()
				continue
			}
			drainBindings = Filter(drainBindings)

			metrics.IncrementCounter("pollCount")

			var totalDrains int
			for _, drainBindings := range drainBindings {
				totalDrains += len(drainBindings.DrainURLs)
			}

			metrics.SendValue("totalDrains", float64(totalDrains), "drains")
			err = store.UpdateDrains(drainBindings)
			if err != nil {
				log.Printf("Error when updating ETCD: %s", err.Error())
				politician.Vacate()
				continue
			}
		}
	}
}
