package lats_test

import (
	"crypto/tls"
	"lats/helpers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	It("gets through recent logs", func() {
		env := &events.Envelope{
			EventType: events.Envelope_LogMessage.Enum(),
			Origin:    proto.String(helpers.ORIGIN_NAME),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			LogMessage: &events.LogMessage{
				AppId:       proto.String("foo"),
				Timestamp:   proto.Int64(time.Now().UnixNano()),
				MessageType: events.LogMessage_OUT.Enum(),
				Message:     []byte("I AM A BANANA!"),
			},
		}
		helpers.EmitToMetron(env)

		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

		getRecentLogs := func() []*events.LogMessage {
			envelopes, err := consumer.RecentLogs("foo", "")
			Expect(err).NotTo(HaveOccurred())
			return envelopes
		}

		Eventually(getRecentLogs).Should(ContainElement(env.LogMessage))
	})
})
