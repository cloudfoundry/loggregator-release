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
	"fmt"
)

var _ = Describe("Streaming logs from an app", func() {
	var appName, appGuid string
	var authToken string

	var doneChan chan struct{}
	var msgChan chan *events.LogMessage
	var errorChan chan error

	const expectedMessageCount = 5

	BeforeEach(func() {
		appName = latsHelpers.PushApp()
		appGuid = latsHelpers.GetAppGuid(appName)
		authToken = latsHelpers.FetchOAuthToken()

		doneChan = make(chan struct{})
		msgChan = make(chan *events.LogMessage, expectedMessageCount)
		errorChan = make(chan error, 5)
		consumerErrorsShouldNotOccur(errorChan, doneChan)
	})

	It("succeeds in sending all log lines", func() {
		defer close(doneChan)

		connection, printer := makeNoaaConsumer()
		defer connection.Close()

		go connection.TailingLogs(appGuid, authToken, msgChan, errorChan)

		waitForWebsocketConnection(printer)

		helpers.CurlApp(appName, fmt.Sprintf("/loglines/%d/LogStreamTestMarker", expectedMessageCount))
		waitForLogMessages(expectedMessageCount, msgChan, appGuid)
	})
})

func makeNoaaConsumer() (*noaa.Consumer, *latsHelpers.TestDebugPrinter) {
	printer := &latsHelpers.TestDebugPrinter{}
	connection := noaa.NewConsumer(latsHelpers.GetDopplerEndpoint(), &tls.Config{InsecureSkipVerify: config.SkipSSLValidation}, nil)
	connection.SetDebugPrinter(printer)
	return connection, printer
}

func consumerErrorsShouldNotOccur(errorChan chan error, doneChan chan struct{}) {
	go func() {
		defer GinkgoRecover()

		select {
		case err := <-errorChan:
			Fail(err.Error())
		case <-doneChan:
		}
	}()
}

func waitForWebsocketConnection(printer *latsHelpers.TestDebugPrinter) {
	Eventually(printer.Dump, 5 * time.Second).Should(ContainSubstring("HTTP/1.1 101 Switching Protocols"))
}

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