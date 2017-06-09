package syslog_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslog"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslogwriter"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SyslogSinkIntegration", func() {
	Describe("HTTPS writer", func() {
		Context("with non-200 return codes", func() {
			PIt("backs off", func() {
				bufferSize := uint(6)
				appId := "appId"
				var timestampsInMillis []int64
				var requests int64

				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(400)
					timestampsInMillis = append(timestampsInMillis, time.Now().UnixNano()/1e6)
					atomic.AddInt64(&requests, 1)
				}))
				url, _ := url.Parse(server.URL)

				dialer := &net.Dialer{}
				httpsWriter, err := syslogwriter.NewHttpsWriter(url, appId, "loggregator", true, dialer, 0)
				Expect(err).ToNot(HaveOccurred())

				errorHandler := func(errorMsg, appId string) {}

				syslogSink := syslog.NewSyslogSink(appId, url, bufferSize, httpsWriter, errorHandler, "dropsonde-origin")
				inputChan := make(chan *events.Envelope)

				defer syslogSink.Disconnect()
				go syslogSink.Run(inputChan)

				for i := 0; i < int(bufferSize); i++ {
					msg := fmt.Sprintf("message number %v", i)
					logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, msg, appId, "App"), "origin")

					inputChan <- logMessage
				}
				close(inputChan)

				Eventually(func() int64 {
					return atomic.LoadInt64(&requests)
				}).Should(BeEquivalentTo(bufferSize))

				// We ignore the difference in timestamps for the 0th iteration because our exponential backoff
				// strategy starts of with a difference of 1 ms
				var diff, prevDiff int64
				for i := 1; i < len(timestampsInMillis)-1; i++ {
					diff = timestampsInMillis[i+1] - timestampsInMillis[i]
					Expect(diff).To(BeNumerically(">", 2*prevDiff))
					prevDiff = diff
				}
			})
		})
	})
})
