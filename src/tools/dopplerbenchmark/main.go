package main
import (
	"runtime"
	"flag"
	"time"
	"log"
	"tools/metronbenchmark/experiment"
	"tools/metronbenchmark/metricsreporter"
	"os"
	"tools/metronbenchmark/messagewriter"
	"tools/dopplerbenchmark/websocketmessagereader"
	"github.com/pivotal-golang/localip"
	"fmt"
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
		log.Fatal("Invalid duration %s\n", *interval)
	}

	stopAfterDuration, err := time.ParseDuration(*stopAfter)
	if err != nil {
		log.Fatal("Invalid duration %s\n", *stopAfter)
	}

	reporter := metricsreporter.New(duration, os.Stdout)
	writer := messagewriter.NewMessageWriter(*dopplerIncomingDropsondePort, sharedSecret, reporter)
	ip, err := localip.LocalIP()
	if err != nil {
		panic(err)
	}
	reader := websocketmessagereader.New(fmt.Sprintf("%s:%d", ip, *dopplerOutgoingPort), reporter)
	defer reader.Close()
	exp := experiment.New(writer, reader, *writeRate)

	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfterDuration)
	<- timer.C
	exp.Stop()
	reporter.Stop()

}