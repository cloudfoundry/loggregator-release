package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"logger"
	"os"
	"os/signal"
	"syscall"
	"time"

	"syslog_drain_binder/elector"
	"syslog_drain_binder/etcd_syslog_drain_store"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	debug       = flag.Bool("debug", false, "Verbose (debug) logging")
	configFile  = flag.String("config", "config/syslog_drain_binder.json", "Location of the Syslog Drain Binder config json file")
)

func main() {
	flag.Parse()
	config, err := parseConfig(*debug, *configFile, *logFilePath)
	if err != nil {
		panic(err)
	}
	log := logger.NewLogger(*debug, *logFilePath, "syslog_drain_binder", config.Syslog)

	dropsonde.Initialize(config.MetronAddress, "syslog_drain_binder")

	workPool, err := workpool.NewWorkPool(config.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: config.EtcdUrls,
	}
	adapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}

	updateInterval := time.Duration(config.UpdateIntervalSeconds) * time.Second
	politician := elector.NewElector(config.InstanceName, adapter, updateInterval, log)

	drainTTL := time.Duration(config.DrainUrlTtlSeconds) * time.Second
	store := etcd_syslog_drain_store.NewEtcdSyslogDrainStore(adapter, drainTTL, log)

	dumpChan := registerGoRoutineDumpSignalChannel()
	ticker := time.NewTicker(updateInterval)
	for {
		select {
		case <-dumpChan:
			logger.DumpGoRoutine()
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

			log.Debugf("Polling %s for updates", config.CloudControllerAddress)
			drainUrls, err := Poll(config.CloudControllerAddress, config.BulkApiUsername, config.BulkApiPassword, config.PollingBatchSize, config.SkipCertVerify)
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

	Syslog string
}

func parseConfig(debug bool, configFile string, logFilePath string) (*Config, error) {
	config := Config{}

	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
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

func registerGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}
