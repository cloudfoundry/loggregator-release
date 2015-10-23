package endtoend_test

import (
	"fmt"
	"integration_tests/helpers"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("App to Syslog Test", func() {

	var (
		appSyslogMap map[*helpers.FakeApp]*helpers.SyslogTCPServer
		registrar    *helpers.SyslogRegistrar
		appCount     int
		logRate      int
		testDuration time.Duration
	)

	runtime.GOMAXPROCS(runtime.NumCPU())

	JustBeforeEach(func() {
		syslogServerPort := 4550
		appCount = 5
		testDuration = 5 * time.Second

		appSyslogMap = make(map[*helpers.FakeApp]*helpers.SyslogTCPServer)

		registrar = helpers.NewSyslogRegistrar(etcdAdapter)

		start := make(chan struct{})
		for i := 0; i < appCount; i++ {
			syslogServer := helpers.NewSyslogTCPServer("127.0.0.1", syslogServerPort+i)
			go syslogServer.Start()

			appID := fmt.Sprintf("app-%d", i)
			app := helpers.NewFakeApp(appID, logRate)
			go app.Start(start)

			appSyslogMap[app] = syslogServer

			registrar.Register(appID, syslogServer.URL())
		}

		close(start)
	})

	AfterEach(func() {
		for app, syslogServer := range appSyslogMap {
			registrar.UnRegister(app.AppID(), syslogServer.URL())
			syslogServer.Stop()
		}
	})

	benchmark := func() {
		Context("low logRate", func() {
			BeforeEach(func() {
				logRate = 100
			})

			Measure("sends the same number of messages for multiple apps to their respective syslog endpoint", func(b Benchmarker) {
				time.Sleep(testDuration)
				for app, syslogServer := range appSyslogMap {
					app.Stop()
					sentLogs := app.SentLogs()
					Eventually(syslogServer.ReceivedLogsRecently, 2).Should(BeFalse())

					receivedLogs := syslogServer.Counter()
					percentLoss := computePercentLost(float64(sentLogs), float64(receivedLogs))
					Expect(percentLoss).To(BeNumerically("<", 0.3))
					b.RecordValue("Sent logs", float64(sentLogs))
					b.RecordValue("Percent Loss", percentLoss)
				}
			}, 1)
		})

		Context("medium logRate", func() {
			BeforeEach(func() {
				logRate = 500
			})

			Measure("message loss for multiple apps logging to respective syslog endpoint", func(b Benchmarker) {
				time.Sleep(testDuration)
				for app, syslogServer := range appSyslogMap {
					app.Stop()
					sentLogs := app.SentLogs()
					Eventually(syslogServer.ReceivedLogsRecently, 2).Should(BeFalse())

					receivedLogs := syslogServer.Counter()
					percentLoss := computePercentLost(float64(sentLogs), float64(receivedLogs))
					Expect(percentLoss).To(BeNumerically("<", 0.5))
					b.RecordValue("Sent logs", float64(sentLogs))
					b.RecordValue("Percent Loss", percentLoss)
				}
			}, 1)
		})

		Context("high logRate", func() {
			BeforeEach(func() {
				logRate = 2000
			})

			Measure("message loss for multiple apps logging to respective syslog endpoint", func(b Benchmarker) {
				time.Sleep(testDuration)
				for app, syslogServer := range appSyslogMap {
					app.Stop()
					sentLogs := app.SentLogs()
					Eventually(syslogServer.ReceivedLogsRecently, 2).Should(BeFalse())

					receivedLogs := syslogServer.Counter()
					percentLoss := computePercentLost(float64(sentLogs), float64(receivedLogs))
					Expect(percentLoss).To(BeNumerically("<", 6))
					b.RecordValue("Sent logs", float64(sentLogs))
					b.RecordValue("Percent Loss", percentLoss)
				}
			}, 1)
		})
	}

	Context("UDP", func() {
		BeforeEach(func() {
			dopplerConfig = "dopplerudp"
		})

		benchmark()
	})

	Context("TLS", func() {
		BeforeEach(func() {
			dopplerConfig = "dopplertls"
		})

		benchmark()
	})

})
