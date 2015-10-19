package main

import (
	"doppler/dopplerservice"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"metron/clientpool"
	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/signer"
	"metron/writers/tagger"

	"logger"
	"metron/eventwriter"
	"runtime"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"metron/config"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

func main() {
	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	log := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)
	log.Info("Startup: Setting up the Metron agent")

	dopplerClientPool, err := initializeClientPool(config, log)
	if err != nil {
		log.Errorf("Error while initializing client pool: %s", err.Error())
		os.Exit(-1)
	}

	dopplerForwarder := dopplerforwarder.New(dopplerClientPool, log)
	byteSigner := signer.New(config.SharedSecret, dopplerForwarder)
	marshaller := eventmarshaller.New(byteSigner, log)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, log)

	initializeMetrics(byteSigner, config, log)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, log)
	dropsondeReader := networkreader.New(fmt.Sprintf("localhost:%d", config.DropsondeIncomingMessagesPort), "dropsondeAgentListener", dropsondeUnmarshaller, log)

	log.Info("metron started")

	dropsondeReader.Start()
}

func initializeClientPool(config *config.Config, logger *gosteno.Logger) (*clientpool.DopplerPool, error) {
	adapter := storeAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		return nil, err
	}

	preferInZone := func(relativePath string) bool {
		return strings.HasPrefix(relativePath, "/"+config.Zone+"/")
	}

	clientPool := clientpool.NewDopplerPool(logger, func(logger *gosteno.Logger, url string) (loggregatorclient.Client, error) {
		client, err := clientpool.NewClient(logger, url)
		if err == nil && client.Scheme() != config.PreferredProtocol {
			logger.Warnd(map[string]interface{}{
				"url": url,
			}, "Doppler advertising UDP only")
		}
		return client, err
	})

	onUpdate := func(all map[string]string, preferred map[string]string) {
		clientPool.Set(all, preferred)
	}

	dopplers, err := dopplerservice.NewFinder(adapter, config.PreferredProtocol, preferInZone, onUpdate, logger)
	if err != nil {
		return nil, err
	}
	dopplers.Start()

	onLegacyUpdate := func(all map[string]string, preferred map[string]string) {
		clientPool.SetLegacy(all, preferred)
	}

	legacyDopplers := dopplerservice.NewLegacyFinder(adapter, config.LoggregatorDropsondePort, preferInZone, onLegacyUpdate, logger)
	legacyDopplers.Start()

	return clientPool, nil
}

func initializeMetrics(byteSigner *signer.Signer, config *config.Config, logger *gosteno.Logger) {
	metricsMarshaller := eventmarshaller.New(byteSigner, logger)
	metricsTagger := tagger.New(config.Deployment, config.Job, config.Index, metricsMarshaller)
	metricsAggregator := messageaggregator.New(metricsTagger, logger)

	eventWriter := eventwriter.New("MetronAgent", metricsAggregator)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalSeconds)*time.Second)
	metrics.Initialize(metricSender, metricBatcher)
}

func storeAdapterProvider(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}

	return etcdAdapter
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
