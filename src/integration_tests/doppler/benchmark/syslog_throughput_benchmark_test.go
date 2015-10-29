package benchmark_test

import (
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/localip"
	"integration_tests/doppler/helpers"
	. "integration_tests/doppler/helpers"
	"io"
	"net"
	"time"
	"tools/benchmark/metricsreporter"
)

var _ = Describe("MessageLossBenchmark", func() {
	Measure("number of messages through a syslog drain per second", func(b Benchmarker) {
		localIP, err := localip.LocalIP()
		Expect(err).NotTo(HaveOccurred())
		syslogDrainAddress := net.JoinHostPort(localIP, "6547")
		listener, err := net.Listen("tcp", syslogDrainAddress)
		Expect(err).NotTo(HaveOccurred())

		counter := metricsreporter.NewCounter("messages")
		go func() {
			conn, err := listener.Accept()
			Expect(err).NotTo(HaveOccurred())

			defer conn.Close()
			buffer := make([]byte, 1024)
			for {
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				_, err := conn.Read(buffer)
				if err == io.EOF {
					return
				} else if err != nil {
					continue
				}

				counter.IncrementValue()
			}
		}()

		guid, _ := uuid.NewV4()
		appID := guid.String()
		syslogDrainURL := "syslog://" + syslogDrainAddress
		key := helpers.DrainKey(appID, syslogDrainURL)
		AddETCDNode(etcdAdapter, key, syslogDrainURL)

		inputConnection, err := net.Dial("udp", localIPAddress+":8765")
		Expect(err).NotTo(HaveOccurred())

		messagePerSecond := 2000
		secondsToRun := 5
		messagesSent := messagePerSecond * secondsToRun
		tolerance := 0.9 // 90%

		writeInterval := time.Second / time.Duration(messagePerSecond)
		ticker := time.NewTicker(writeInterval)
		for i := 0; i < messagesSent; i++ {
			select {
			case <-ticker.C:
				SendAppLog(appID, "syslog-message", inputConnection)
			}
		}

		Eventually(counter.GetValue).Should(BeNumerically(">", float64(messagesSent)*tolerance))
		b.RecordValue("messages received", float64(counter.GetValue()))

		listener.Close()
	}, 3)
})
