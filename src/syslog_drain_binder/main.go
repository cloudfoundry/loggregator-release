package main

import (
	"flag"
	"syslog_drain_binder/elector"
	"syslog_drain_binder/etcd_syslog_drain_store"
	"time"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var (
	debug      = flag.Bool("debug", false, "Verbose (debug) logging")
	configFile = flag.String("config", "config/syslog_drain_binder.json", "Location of the Syslog Drain Binder config json file")
)

func main() {
	flag.Parse()
	config := parseConfig(*configFile)
	logger := cfcomponent.NewLogger(*debug, "", "syslog_drain_binder", config.Config)

	workPool := workpool.NewWorkPool(config.EtcdConcurrentRequests)
	adapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workPool)

	updateInterval := time.Duration(config.UpdateIntervalSeconds) * time.Second
	politician := elector.NewElector(config.InstanceName, adapter, updateInterval, logger)

	drainTTL := time.Duration(config.DrainUrlTtlSeconds) * time.Second
	store := etcd_syslog_drain_store.NewEtcdSyslogDrainStore(adapter, drainTTL, logger)

	var err error
	ticker := time.NewTicker(updateInterval)
	for {
		<-ticker.C

		if politician.IsLeader() {
			err = politician.StayAsLeader()
			if err != nil {
				logger.Errorf("Error when staying leader: %s", err.Error())
				politician.Vacate()
				continue
			}
		} else {
			err = politician.RunForElection()

			if err != nil {
				logger.Errorf("Error when running for leader: %s", err.Error())
				politician.Vacate()
				continue
			}
		}

		logger.Debugf("Polling %s for updates", config.CloudControllerAddress)
		drainUrls, err := Poll(config.CloudControllerAddress, config.BulkApiUsername, config.BulkApiPassword, config.PollingBatchSize)
		if err != nil {
			logger.Errorf("Error when polling cloud controller: %s", err.Error())
			politician.Vacate()
			continue
		}

		logger.Debugf("Updating drain URLs for %d application(s)", len(drainUrls))
		err = store.UpdateDrains(drainUrls)
		if err != nil {
			logger.Errorf("Error when updating ETCD: %s", err.Error())
			politician.Vacate()
			continue
		}
	}
}

type Config struct {
	InstanceName          string
	DrainUrlTtlSeconds    int64
	UpdateIntervalSeconds int64

	EtcdConcurrentRequests int
	EtcdUrls               []string

	CloudControllerAddress string
	BulkApiUsername        string
	BulkApiPassword        string
	PollingBatchSize       int

	cfcomponent.Config
}

func parseConfig(configFile string) Config {
	config := Config{}

	err := cfcomponent.ReadConfigInto(&config, configFile)
	if err != nil {
		panic(err)
	}

	return config
}

var StoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool := workpool.NewWorkPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
}
