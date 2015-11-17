package main

import (
	"crypto/tls"
	"doppler/dopplerservice"
	"doppler/listeners"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"metron/clientpool"
	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/tagger"

	"logger"
	"metron/eventwriter"
	"runtime"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"github.com/cloudfoundry/sonde-go/events"
	"metron/config"
	"truncatingbuffer"
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

	dopplerClientPool, err := initializeDopplerPool(config, log)
	if err != nil {
		log.Errorf("Failed to initialize the doppler pool: %s", err.Error())
		os.Exit(-1)
	}

	inputChan := make(<-chan *events.Envelope)
	stopChan := make(chan struct{})
	bufferContext := truncatingbuffer.NewDefaultContext("metron", "")
	truncatingBuffer := truncatingbuffer.NewTruncatingBuffer(inputChan, 100, bufferContext, log, stopChan)
	dopplerForwarder := dopplerforwarder.New(dopplerClientPool, []byte(config.SharedSecret), truncatingBuffer, log)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, dopplerForwarder)
	aggregator := messageaggregator.New(messageTagger, log)

	initializeMetrics(messageTagger, config, log)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, log)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.DropsondeIncomingMessagesPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller, log)
	if err != nil {
		log.Errorf("Failed to listen on %s: %s", metronAddress, err)
		os.Exit(1)
	}

	log.Info("metron started")

	dropsondeReader.Start()
}

func initializeDopplerPool(config *config.Config, logger *gosteno.Logger) (*clientpool.DopplerPool, error) {
	adapter, err := storeAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	if err != nil {
		return nil, err
	}
	err = adapter.Connect()
	if err != nil {
		logger.Warnd(map[string]interface{}{
			"error": err.Error(),
		}, "Failed to connect to etcd")
	}

	preferInZone := func(relativePath string) bool {
		return strings.HasPrefix(relativePath, "/"+config.Zone+"/")
	}

	var tlsConfig *tls.Config
	if config.PreferredProtocol == "tls" {
		c := config.TLSConfig
		tlsConfig, err = listeners.NewTLSConfig(c.CertFile, c.KeyFile, c.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ServerName = "doppler"
	}

	clientPool := clientpool.NewDopplerPool(logger, func(logger *gosteno.Logger, url string) (clientpool.Client, error) {
		client, err := clientpool.NewClient(logger, url, tlsConfig)
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

func initializeMetrics(messageTagger *tagger.Tagger, config *config.Config, logger *gosteno.Logger) {
	metricsAggregator := messageaggregator.New(messageTagger, logger)

	eventWriter := eventwriter.New("MetronAgent", metricsAggregator)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalSeconds)*time.Second)
	metrics.Initialize(metricSender, metricBatcher)
}

func storeAdapterProvider(urls []string, concurrentRequests int) (storeadapter.StoreAdapter, error) {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		return nil, err
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		return nil, err
	}

	return etcdAdapter, nil
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
