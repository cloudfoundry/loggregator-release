package integration_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregator_consumer"
	"github.com/cloudfoundry/noaa/consumer"
)

const accessLogFile = "/tmp/access.log"

var _ = Describe("TrafficController's access logs", func() {
	var (
		testContents func() string
	)

	BeforeEach(func() {
		configFile = "fixtures/trafficcontroller_access_log.json"

		testFile, err := os.Create(accessLogFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(testFile.Close()).To(Succeed())

		testContents = func() string {
			contents, err := ioutil.ReadFile(accessLogFile)
			Expect(err).ToNot(HaveOccurred())
			return string(contents)
		}
	})

	AfterEach(func() {
		Expect(os.Remove(accessLogFile)).To(Succeed())
	})

	Context("with modern endpoints", func() {
		var noaaConsumer *consumer.Consumer

		JustBeforeEach(func() {
			tcURL := fmt.Sprintf("ws://%s:%d", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT)
			noaaConsumer = consumer.New(tcURL, &tls.Config{}, nil)
		})

		AfterEach(func() {
			noaaConsumer.Close()
		})

		It("logs stream access", func() {
			noaaConsumer.Stream(APP_ID, AUTH_TOKEN)

			expected := fmt.Sprintf("CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /apps/%s/stream|GET /apps/%[1]s/stream|0|", APP_ID)
			Eventually(testContents).Should(ContainSubstring(expected))
		})

		It("logs recent access", func() {
			noaaConsumer.RecentLogs(APP_ID, AUTH_TOKEN)

			expected := fmt.Sprintf("CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /apps/%s/recentlogs|GET /apps/%[1]s/recentlogs|0|", APP_ID)
			Eventually(testContents).Should(ContainSubstring(expected))
		})

		It("logs container metrics access", func() {
			noaaConsumer.ContainerMetrics(APP_ID, AUTH_TOKEN)

			expected := fmt.Sprintf("CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /apps/%s/containermetrics|GET /apps/%[1]s/containermetrics|0|", APP_ID)
			Eventually(testContents).Should(ContainSubstring(expected))
		})

		It("logs firehose access", func() {
			noaaConsumer.Firehose("foo", AUTH_TOKEN)

			expected := "CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /firehose/foo|GET /firehose/foo|0|"
			Eventually(testContents).Should(ContainSubstring(expected))
		})
	})

	Context("with legacy endpoints", func() {
		var legacyConsumer loggregator_consumer.LoggregatorConsumer

		JustBeforeEach(func() {
			tcURL := fmt.Sprintf("ws://%s:%d", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT)
			legacyConsumer = loggregator_consumer.New(tcURL, &tls.Config{}, nil)
		})

		AfterEach(func() {
			legacyConsumer.Close()
		})

		It("logs tail access", func() {
			legacyConsumer.Tail(APP_ID, AUTH_TOKEN)

			expected := fmt.Sprintf("CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /tail/?app=%s|GET /tail/?app=%[1]s|0|", APP_ID)
			Eventually(testContents).Should(ContainSubstring(expected))
		})

		It("logs recent access", func() {
			legacyConsumer.Recent(APP_ID, AUTH_TOKEN)

			expected := fmt.Sprintf("CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /recent?app=%s|GET /recent?app=%[1]s|0|", APP_ID)
			Eventually(testContents).Should(ContainSubstring(expected))
		})
	})
})
