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

var _ = Describe("TrafficController's access logs", func() {
	var (
		testContents func() string
	)

	BeforeEach(func() {
		testFile, err := ioutil.TempFile(os.TempDir(), "access_test_")
		Expect(err).ToNot(HaveOccurred())
		accessLogFile = testFile.Name()
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

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/apps/%s/stream\n", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT, APP_ID)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
		})

		It("logs recent access", func() {
			noaaConsumer.RecentLogs(APP_ID, AUTH_TOKEN)

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/apps/%s/recentlogs\n", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT, APP_ID)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
		})

		It("logs container metrics access", func() {
			noaaConsumer.ContainerMetrics(APP_ID, AUTH_TOKEN)

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/apps/%s/containermetrics\n", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT, APP_ID)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
		})

		It("logs firehose access", func() {
			noaaConsumer.Firehose("foo", AUTH_TOKEN)

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/firehose/foo\n", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
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

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/tail/\n", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
		})

		It("logs recent access", func() {
			legacyConsumer.Recent(APP_ID, AUTH_TOKEN)

			expectedSuffix := fmt.Sprintf("GET: http://%s:%d/recent\n", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT)
			Eventually(testContents).Should(HaveSuffix(expectedSuffix))
		})
	})
})
