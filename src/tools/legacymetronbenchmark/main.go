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

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"
	"tools/legacymetronbenchmark/legacyreader"
)

func main() {
	runtime.GOMAXPROCS(4)

	var interval = flag.String("interval", "1s", "Interval for reported results")
	var writeRate = flag.Int("writeRate", 15000, "Number of writes per second to send to metron")
	var stopAfter = flag.String("stopAfter", "5m", "How long to run the experiment for")
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

	reader := messagereader.NewMessageReader(3457)
	legacyReader := legacyreader.NewLegacyReader(reporter, reader)

	exp := experiment.NewExperiment(legacyReader)

	generator := messagegenerator.NewValueMetricGenerator()
	writer := messagewriter.NewMessageWriter("localhost", 51161, "", reporter)
	writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, *writeRate)
	exp.AddWriteStrategy(writeStrategy)

	legacyGenerator := messagegenerator.NewLegacyLogGenerator()
	legacyWriter := messagewriter.NewMessageWriter("localhost", 51160, "", reporter)
	legacyWriteStrategy := writestrategies.NewConstantWriteStrategy(legacyGenerator, legacyWriter, *writeRate)
	exp.AddWriteStrategy(legacyWriteStrategy)

	announceToEtcd()

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
	etcdStoreAdapter := etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
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
