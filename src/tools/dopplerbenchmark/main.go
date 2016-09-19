package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/messagewriter"
	"tools/benchmark/metricsreporter"
	"tools/benchmark/writestrategies"
	"tools/dopplerbenchmark/websocketmessagereader"

	"code.cloudfoundry.org/localip"
)

func main() {
	runtime.GOMAXPROCS(4)

	var interval = flag.String("interval", "1s", "Interval for reported results")
	var writeRate = flag.Int("writeRate", 15000, "Number of writes per second to send to doppler")
	var stopAfter = flag.String("stopAfter", "5m", "How long to run the experiment for")
	var sharedSecret string
	flag.StringVar(&sharedSecret, "sharedSecret", "", "Shared secret used by Doppler to verify message validity")
	var dopplerOutgoingPort = flag.Int("dopplerOutgoingPort", 8080, "Outgoing port from doppler")
	var dopplerIncomingDropsondePort = flag.Int("dopplerIncomingDropsondePort", 3457, "Incoming dropsonde port to doppler")
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
	ip, err := localip.LocalIP()
	if err != nil {
		panic(err)
	}

	generator := messagegenerator.NewValueMetricGenerator()
	writer := messagewriter.NewMessageWriter(ip, *dopplerIncomingDropsondePort, sharedSecret, reporter.SentCounter())
	reader := websocketmessagereader.New(fmt.Sprintf("%s:%d", ip, *dopplerOutgoingPort), reporter.ReceivedCounter())
	defer reader.Close()

	writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, *writeRate)
	exp := experiment.NewExperiment(reader)
	exp.AddWriteStrategy(writeStrategy)

	exp.Warmup()
	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfterDuration)
	<-timer.C
	exp.Stop()
	reporter.Stop()
}
