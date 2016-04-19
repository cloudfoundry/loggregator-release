package accesslogger_test

import (
	"errors"
	"trafficcontroller/accesslogger"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessLogger", func() {
	var (
		writer *mockWriter
		logger *accesslogger.AccessLogger
	)

	BeforeEach(func() {
		writer = newMockWriter()
		loggie := loggertesthelper.Logger()
		logger = accesslogger.New(writer, loggie)
	})

	Describe("LogAccess", func() {
		Context("Happy", func() {
			BeforeEach(func() {
				writer.WriteOutput.Err <- nil
				writer.WriteOutput.Sent <- 0
			})

			It("Logs Access", func() {
				req, err := newServerRequest("GET", "some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(logger.LogAccess(req, "1.1.1.1", 1)).To(Succeed())
				prefix := "CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET some.url.com/foo|GET some.url.com/foo|0|"
				Expect(writer.WriteInput.Message).To(Receive(HavePrefix(prefix)))
			})

			It("Includes details about the access", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"

				Expect(logger.LogAccess(req, "1.1.1.1", 1)).To(Succeed())
				Expect(writer.WriteInput.Message).To(Receive(ContainSubstring("src=127.0.0.1 spt=4567")))
			})

			It("Uses X-Forwarded-For if it exists", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"
				req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

				Expect(logger.LogAccess(req, "1.1.1.1", 1)).To(Succeed())
				Expect(writer.WriteInput.Message).To(Receive(ContainSubstring("src=50.60.70.80 spt=1234")))
			})

			It("Writes multiple log lines", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"

				Expect(logger.LogAccess(req, "1.1.1.1", 1)).To(Succeed())

				writer.WriteOutput.Err <- nil
				writer.WriteOutput.Sent <- 0
				req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")
				Expect(logger.LogAccess(req, "1.1.1.1", 1)).To(Succeed())

				var (
					log      []byte
					expected string
				)
				expected = "src=127.0.0.1 spt=4567"
				Expect(writer.WriteInput.Message).To(Receive(&log))
				Expect(log).To(ContainSubstring(expected))
				Expect(log).To(HaveSuffix("\n"))
				expected = "src=50.60.70.80 spt=1234"
				Expect(writer.WriteInput.Message).To(Receive(&log))
				Expect(log).To(ContainSubstring(expected))
				Expect(log).To(HaveSuffix("\n"))
			})
		})

		Context("Unhappy", func() {
			BeforeEach(func() {
				writer.WriteOutput.Err <- errors.New("boom")
				writer.WriteOutput.Sent <- 0
			})

			It("Errors", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"
				Expect(logger.LogAccess(req, "1.1.1.1", 1)).ToNot(Succeed())
			})
		})
	})
})
