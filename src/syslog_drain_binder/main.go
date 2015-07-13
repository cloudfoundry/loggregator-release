package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"syslog_drain_binder/elector"
	"syslog_drain_binder/etcd_syslog_drain_store"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	debug       = flag.Bool("debug", false, "Verbose (debug) logging")
	configFile  = flag.String("config", "config/syslog_drain_binder.json", "Location of the Syslog Drain Binder config json file")
)

func main() {
	flag.Parse()
	config, logger := parseConfig(*debug, *configFile, *logFilePath)

	dropsonde.Initialize(config.MetronAddress, "syslog_drain_binder")

	workPool, err := workpool.NewWorkPool(config.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}

	adapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workPool)

	updateInterval := time.Duration(config.UpdateIntervalSeconds) * time.Second
	politician := elector.NewElector(config.InstanceName, adapter, updateInterval, logger)

	drainTTL := time.Duration(config.DrainUrlTtlSeconds) * time.Second
	store := etcd_syslog_drain_store.NewEtcdSyslogDrainStore(adapter, drainTTL, logger)

	ticker := time.NewTicker(updateInterval)
	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-ticker.C:
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
			drainUrls, err := Poll(config.CloudControllerAddress, config.BulkApiUsername, config.BulkApiPassword, config.PollingBatchSize, config.SkipCertVerify)
			if err != nil {
				logger.Errorf("Error when polling cloud controller: %s", err.Error())
				politician.Vacate()
				continue
			}

			metrics.IncrementCounter("pollCount")

			var totalDrains int
			for _, drainList := range drainUrls {
				totalDrains += len(drainList)
			}

			metrics.SendValue("totalDrains", float64(totalDrains), "drains")

			logger.Debugf("Updating drain URLs for %d application(s)", len(drainUrls))
			err = store.UpdateDrains(drainUrls)
			if err != nil {
				logger.Errorf("Error when updating ETCD: %s", err.Error())
				politician.Vacate()
				continue
			}
		}
	}
}

type Config struct {
	InstanceName          string
	DrainUrlTtlSeconds    int64
	UpdateIntervalSeconds int64

	EtcdMaxConcurrentRequests int
	EtcdUrls                  []string

	MetronAddress string

	CloudControllerAddress string
	BulkApiUsername        string
	BulkApiPassword        string
	PollingBatchSize       int

	SkipCertVerify bool

	cfcomponent.Config
}

func parseConfig(debug bool, configFile string, logFilePath string) (Config, *gosteno.Logger) {
	config := Config{}
	err := cfcomponent.ReadConfigInto(&config, configFile)
	if err != nil {
		panic(err)
	}

	err = config.validate()
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(debug, logFilePath, "syslog_drain_binder", config.Config)

	return config, logger
}

func (config Config) validate() error {
	if config.MetronAddress == "" {
		return errors.New("Need Metron address (host:port).")
	}

	if config.EtcdMaxConcurrentRequests < 1 {
		return fmt.Errorf("Need EtcdMaxConcurrentRequests ≥ 1, received %d", config.EtcdMaxConcurrentRequests)
	}

	return nil
}
