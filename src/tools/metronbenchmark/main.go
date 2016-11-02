package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/messagereader"
	"tools/benchmark/messagewriter"
	"tools/benchmark/metricsreporter"
	"tools/benchmark/writestrategies"
	"tools/metronbenchmark/eventtypereader"

	"plumbing"
)

const nodeKey = "/doppler/meta/z1/doppler_z1/0"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		interval          time.Duration
		writeRate         int
		stopAfter         time.Duration
		concurrentWriters int
		eventType         events.Envelope_EventType
		protocol          string
		serverCert        string
		serverKey         string
		caCert            string
		burstDelay        time.Duration
	)

	flag.Var(newDurationValue(&interval, time.Second), "interval", "Interval for reported results")
	flag.IntVar(&writeRate, "writeRate", 15000, "Number of writes per second to send to metron")
	flag.Var(newDurationValue(&stopAfter, 5*time.Minute), "stopAfter", "How long to run the experiment for")
	flag.IntVar(&concurrentWriters, "concurrentWriters", 1, "Number of concurrent writers")
	flag.Var(newEventTypeValue(&eventType, events.Envelope_ValueMetric), "eventType", "The event type to test")
	flag.StringVar(&protocol, "protocol", "udp", "The protocol to configure metron to send messages over")
	flag.StringVar(&serverCert, "serverCert", "../../integration_tests/fixtures/server.crt", "The server cert file (for TLS connections)")
	flag.StringVar(&serverKey, "serverKey", "../../integration_tests/fixtures/server.key", "The server key file (for TLS connections)")
	flag.StringVar(&caCert, "caCert", "../../integration_tests/fixtures/loggregator-ca.crt", "The certificate authority cert file (for TLS connections)")
	flag.Var(newDurationValue(&burstDelay, 0), "burstDelay", "The delay between burst sequences.  If this is non-zero then writeRate is used as the number of messages to send each burst.")

	flag.Parse()

	reporter := metricsreporter.New(interval, os.Stdout)
	var generator writestrategies.MessageGenerator
	switch eventType {
	case events.Envelope_ValueMetric:
		generator = messagegenerator.NewValueMetricGenerator()
	case events.Envelope_LogMessage:
		generator = messagegenerator.NewLogMessageGenerator("fake-app-id")
	default:
		log.Fatalf("Unsupported envelope type: %v", eventType)
	}
	var dopplerURLs []string
	var reader eventtypereader.MessageReader
	switch protocol {
	case "udp":
		reader = messagereader.NewUDP(3457)
		dopplerURLs = []string{"udp://127.0.0.1:3457"}
	case "tls":
		tlsConfig, err := plumbing.NewTLSConfig(
			serverCert,
			serverKey,
			caCert,
			"",
		)
		if err != nil {
			log.Printf("Error: failed to load TLS config: %s", err)
			os.Exit(1)
		}
		tlsConfig.InsecureSkipVerify = true
		reader = messagereader.NewTLS(3458, tlsConfig)
		dopplerURLs = []string{"tls://127.0.0.1:3458"}
	default:
		panic(fmt.Errorf("Unknown protocol %s", protocol))
	}
	valueMetricReader := eventtypereader.New(reporter.ReceivedCounter(), reader, eventType, "test-origin")
	exp := experiment.NewExperiment(valueMetricReader)

	for i := 0; i < concurrentWriters; i++ {
		writer := messagewriter.NewMessageWriter("localhost", 51161, "", reporter.SentCounter())
		writeStrategy := chooseStrategy(generator, writer, writeRate, burstDelay)
		exp.AddWriteStrategy(writeStrategy)
	}

	adapter := announceToEtcd(dopplerURLs...)
	defer func() {
		exp.Stop()
		reporter.Stop()
		err := adapter.Delete(nodeKey)
		if err != nil {
			log.Printf("Warning: Failed to delete etcd key %s: %s", nodeKey, err)
		}
		adapter.Disconnect()
	}()

	exp.Warmup()
	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfter)
	<-timer.C
}

func chooseStrategy(generator writestrategies.MessageGenerator, writer writestrategies.MessageWriter, writeRate int, burstDelay time.Duration) experiment.WriteStrategy {
	if burstDelay > 0 {
		params := writestrategies.BurstParameters{
			Minimum:   writeRate,
			Maximum:   writeRate,
			Frequency: burstDelay,
		}
		return writestrategies.NewBurstWriteStrategy(generator, writer, params)
	}
	return writestrategies.NewConstantWriteStrategy(generator, writer, writeRate)
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
	etcdStoreAdapter.Connect()
	return etcdStoreAdapter
}

func announceToEtcd(addresses ...string) storeadapter.StoreAdapter {
	etcdUrls := []string{"http://localhost:4001"}
	etcdMaxConcurrentRequests := 10
	storeAdapter := NewStoreAdapter(etcdUrls, etcdMaxConcurrentRequests)
	addressJSON, err := json.Marshal(map[string]interface{}{
		"endpoints": addresses,
	})
	if err != nil {
		panic(err)
	}
	node := storeadapter.StoreNode{
		Key:   nodeKey,
		Value: addressJSON,
	}
	err = storeAdapter.Create(node)
	if err != nil {
		panic(err)
	}
	time.Sleep(50 * time.Millisecond)
	return storeAdapter
}

// durationValue is a flag.Value for a time.Duration type
type durationValue time.Duration

func newDurationValue(duration *time.Duration, value time.Duration) *durationValue {
	*duration = value
	return (*durationValue)(duration)
}

func (d *durationValue) String() string {
	return time.Duration(*d).String()
}

func (d *durationValue) Set(s string) error {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = durationValue(duration)
	return nil
}

// eventTypeValue is a flag.Value for an events.Envelope_EventType type
type eventTypeValue events.Envelope_EventType

func newEventTypeValue(typ *events.Envelope_EventType, value events.Envelope_EventType) *eventTypeValue {
	*typ = value
	return (*eventTypeValue)(typ)
}

func (e *eventTypeValue) String() string {
	return events.Envelope_EventType(*e).String()
}

func (e *eventTypeValue) Set(s string) error {
	typ, ok := events.Envelope_EventType_value[s]
	if !ok {
		return fmt.Errorf("No event type %s found", s)
	}
	*e = eventTypeValue(typ)
	return nil
}
