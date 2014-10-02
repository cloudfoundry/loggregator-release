package marshaller_test

import (
	"trafficcontroller/marshaller"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoggregatorLogMessage", func() {
	It("unmarshals as a loggregator LogMessage", func() {
		msg := marshaller.LoggregatorLogMessage("hello", "abc123")

		logMessage := &logmessage.LogMessage{}
		err := proto.Unmarshal(msg, logMessage)
		Expect(err).NotTo(HaveOccurred())

		Expect(logMessage.GetMessage()).To(BeEquivalentTo("hello"))
		Expect(logMessage.GetAppId()).To(Equal("abc123"))
	})
})

var _ = Describe("DropsondeLogMessage", func() {
	It("unmarshals as a dropsonde envelope containing a log message", func() {
		msg := marshaller.DropsondeLogMessage("hello", "abc123")

		envelope := &events.Envelope{}
		err := proto.Unmarshal(msg, envelope)
		Expect(err).NotTo(HaveOccurred())

		Expect(envelope.GetEventType()).To(Equal(events.Envelope_LogMessage))
		Expect(dropsonde.GetAppId(envelope)).To(Equal("abc123"))

		logMessage := envelope.GetLogMessage()
		Expect(logMessage.GetMessage()).To(BeEquivalentTo("hello"))
		Expect(logMessage.GetSourceType()).To(Equal("DOP"))
		Expect(logMessage.GetMessageType()).To(Equal(events.LogMessage_ERR))
	})
})
