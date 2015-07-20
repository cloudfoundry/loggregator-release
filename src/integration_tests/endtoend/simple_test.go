package endtoend_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"time"
	"tools/benchmark/experiment"
	"integration_tests/endtoend"
)

const (
	numMessagesSent = 1000
	timeoutSeconds  = 15
)

var _ = Describe("Simple test", func() {
	Measure("dropsonde metrics being passed from metron to the firehose nozzle", func(b Benchmarker) {
		metronConn := initiateMetronConnection()

		metronStreamWriter := endtoend.NewMetronStreamWriter(metronConn, numMessagesSent)

		firehosereader := endtoend.NewFirehoseReader(b, numMessagesSent)

		ex := experiment.New(metronStreamWriter, firehosereader, numMessagesSent)

		go stopExperimentAfterTimeout(ex)

		b.Time("runtime", func() {
			ex.Start()
		})

		firehosereader.Report()
	}, 3)
})

func stopExperimentAfterTimeout(ex *experiment.Experiment){
		t := time.NewTicker(time.Duration(timeoutSeconds) * time.Second)
		<- t.C
		ex.Stop()
}

func initiateMetronConnection() net.Conn {
	metronInput, err := net.Dial("udp", "localhost:49625")
	Expect(err).ToNot(HaveOccurred())
	return metronInput
}

