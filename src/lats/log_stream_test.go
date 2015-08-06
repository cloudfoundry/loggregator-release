package lats_test

import (
	"crypto/tls"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
	latsHelpers "lats/helpers"
)

var _ = Describe("Streaming logs from an app", func() {
	var appName string
	var authToken string

	BeforeEach(func() {
		appName = latsHelpers.PushApp()
		authToken = latsHelpers.FetchOAuthToken()
	})

	It("succeeds in sending all log lines", func() {
		doneChan := make(chan struct{})
		errorChan := make(chan error, 5)
		go func() {
			defer GinkgoRecover()

			select {
			case err := <-errorChan:
				Fail(err.Error())
			case <-doneChan:
			}
		}()

		msgChan := make(chan *events.LogMessage)
		printer := &latsHelpers.TestDebugPrinter{}
		connection := noaa.NewConsumer(latsHelpers.GetDopplerEndpoint(), &tls.Config{InsecureSkipVerify: config.SkipSSLValidation}, nil)
		connection.SetDebugPrinter(printer)
		defer connection.Close()

		appGuid := latsHelpers.GetAppGuid(appName)
		go connection.TailingLogs(appGuid, authToken, msgChan, errorChan)

		// Make sure the websocket connection is ready
		Eventually(printer.Dump, 5 * time.Second).Should(ContainSubstring("HTTP/1.1 101 Switching Protocols"))

		// Make app log some logs
		helpers.CurlApp(appName, "/loglines/5/LogStreamTestMarker")

		// Expect all logs to appear in Noaa consumer
		waitForLogMessages(5, msgChan, appGuid)

		close(doneChan)
	})
})

func waitForLogMessages(expectedMessageCount int, msgChan chan *events.LogMessage, appGuid string) {
	var recvEnvelope *events.LogMessage
	count := 0

	for count < expectedMessageCount {
		Eventually(msgChan).Should(Receive(&recvEnvelope))
		Expect(recvEnvelope.GetAppId()).To(Equal(appGuid))
		Expect(string(recvEnvelope.GetMessage())).To(ContainSubstring("LogStreamTestMarker"))
		count++
	}
}