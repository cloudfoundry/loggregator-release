package main

import (
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagewriter"

	"tools/benchmark/messagereader"

	"os"
	"tools/benchmark/metricsreporter"

	"flag"
	"log"

	"runtime"

	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"
	"tools/metronbenchmark/valuemetricreader"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var interval = flag.String("interval", "1s", "Interval for reported results")
	var writeRate = flag.Int("writeRate", 15000, "Number of writes per second to send to metron")
	var stopAfter = flag.String("stopAfter", "5m", "How long to run the experiment for")
	var concurrentWriters = flag.Int("concurrentWriters", 1, "Number of concurrent writers")

	flag.Parse()

	duration, err := time.ParseDuration(*interval)
	if err != nil {
		log.Fatalf("Invalid duration %s\n", *interval)
	}

	stopAfterDuration, err := time.ParseDuration(*stopAfter)
	if err != nil {
		log.Fatalf("Invalid duration %s\n", *stopAfter)
	}

	reporter := metricsreporter.New(duration, os.Stdout)
	generator := messagegenerator.NewValueMetricGenerator()
	reader := messagereader.NewMessageReader(3457)
	valueMetricReader := valuemetricreader.NewValueMetricReader(reporter.GetReceivedCounter(), reader)
	exp := experiment.NewExperiment(valueMetricReader)

	for i := 0; i < *concurrentWriters; i++ {
		writer := messagewriter.NewMessageWriter("localhost", 51161, "", reporter.GetSentCounter())
		writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, *writeRate)
		exp.AddWriteStrategy(writeStrategy)
	}

	announceToEtcd()

	exp.Warmup()
	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfterDuration)
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
