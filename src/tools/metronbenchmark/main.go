package main

import (
	"fmt"
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagewriter"
	"tools/metronbenchmark/eventtypereader"

	"tools/benchmark/messagereader"

	"os"
	"tools/benchmark/metricsreporter"

	"flag"
	"log"

	"runtime"

	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		interval          time.Duration
		writeRate         int
		stopAfter         time.Duration
		concurrentWriters int
		eventType         events.Envelope_EventType
	)
	flag.Var(newDurationValue(&interval, time.Second), "interval", "Interval for reported results")
	flag.IntVar(&writeRate, "writeRate", 15000, "Number of writes per second to send to metron")
	flag.Var(newDurationValue(&stopAfter, 5*time.Minute), "stopAfter", "How long to run the experiment for")
	flag.IntVar(&concurrentWriters, "concurrentWriters", 1, "Number of concurrent writers")
	flag.Var(newEventTypeValue(&eventType, events.Envelope_ValueMetric), "eventType", "The event type to test")

	flag.Parse()

	reporter := metricsreporter.New(stopAfter, os.Stdout)
	var generator writestrategies.MessageGenerator
	switch eventType {
	case events.Envelope_ValueMetric:
		generator = messagegenerator.NewValueMetricGenerator()
	case events.Envelope_LogMessage:
		generator = messagegenerator.NewLogMessageGenerator("fake-app-id")
	default:
		log.Fatalf("Unsupported envelope type: %v", eventType)
	}
	reader := messagereader.New(3457)
	valueMetricReader := eventtypereader.New(reporter.GetReceivedCounter(), reader, eventType, "test-origin")
	exp := experiment.NewExperiment(valueMetricReader)

	for i := 0; i < concurrentWriters; i++ {
		writer := messagewriter.NewMessageWriter("localhost", 51161, "", reporter.GetSentCounter())
		writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, writeRate)
		exp.AddWriteStrategy(writeStrategy)
	}

	announceToEtcd()

	exp.Warmup()
	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfter)
	<-timer.C
	exp.Stop()
	reporter.Stop()
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

func announceToEtcd() {
	etcdUrls := []string{"http://localhost:4001"}
	etcdMaxConcurrentRequests := 10
	storeAdapter := NewStoreAdapter(etcdUrls, etcdMaxConcurrentRequests)
	node := storeadapter.StoreNode{
		Key:   "/healthstatus/doppler/z1/0",
		Value: []byte("localhost"),
	}
	storeAdapter.Create(node)
	storeAdapter.Disconnect()
	time.Sleep(50 * time.Millisecond)
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
