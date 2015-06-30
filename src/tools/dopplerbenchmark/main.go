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
)

func main() {
	runtime.GOMAXPROCS(4)

	var interval = flag.String("interval", "1s", "Interval for reported results")
	var writeRate = flag.Int("writeRate", 15000, "Number of writes per second to send to doppler")
	var stopAfter = flag.String("stopAfter", "5m", "How long to run the experiment for")
	var sharedSecret string
	flag.StringVar(&sharedSecret, "sharedSecret", "", "Shared secret used by Doppler to verify message validity")
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
	writer := messagewriter.NewMessageWriter(3457, sharedSecret, reporter)
	ip, err := localip.LocalIP()
	if err != nil {
		panic(err)
	}
	reader := websocketmessagereader.New(ip+":8080", reporter)
	exp := experiment.New(writer, reader, *writeRate)

	go reporter.Start()
	go exp.Start()

	timer := time.NewTimer(stopAfterDuration)
	<- timer.C
	exp.Stop()
	reporter.Stop()
}