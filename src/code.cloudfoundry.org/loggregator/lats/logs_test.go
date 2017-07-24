package lats_test

import (
	"crypto/tls"
	"time"

	"code.cloudfoundry.org/loggregator/lats/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/golang/protobuf/proto"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	Describe("emit v1 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			env := createLogEnvelopeV1("Recent log message", "foo")
			helpers.EmitToMetronV1(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs("foo", "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}

			Eventually(getRecentLogs).Should(ContainElement(env.LogMessage))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			msgChan, errorChan := helpers.ConnectToStream("foo")

			env := createLogEnvelopeV1("Stream message", "foo")
			helpers.EmitToMetronV1(env)

			receivedEnvelope, err := helpers.FindMatchingEnvelopeByID("foo", msgChan)
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedEnvelope.LogMessage).To(Equal(env.LogMessage))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v2 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			env := createLogEnvelopeV2("Recent log message", "foo")
			helpers.EmitToMetronV2(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs("foo", "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}
			v1Env := conversion.ToV1(env)[0]
			Eventually(getRecentLogs).Should(ContainElement(v1Env.LogMessage))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			msgChan, errorChan := helpers.ConnectToStream("foo-stream")

			env := createLogEnvelopeV2("Stream message", "foo-stream")
			helpers.EmitToMetronV2(env)

			receivedEnvelope, err := helpers.FindMatchingEnvelopeByID("foo-stream", msgChan)
			Expect(err).NotTo(HaveOccurred())

			v1Env := conversion.ToV1(env)[0]
			Expect(receivedEnvelope.LogMessage).To(Equal(v1Env.LogMessage))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v1 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			msgChan := helpers.ReadFromRLP("rlp-stream-foo", false)

			env := createLogEnvelopeV1("Stream message", "rlp-stream-foo")
			helpers.EmitToMetronV1(env)

			v2Env := conversion.ToV2(env, false)

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(v2Env.GetLog()))
		})

		It("sends log messages through rlp with preferred tags", func() {
			msgChan := helpers.ReadFromRLP("rlp-stream-foo", true)

			env := createLogEnvelopeV1("Stream message", "rlp-stream-foo")
			helpers.EmitToMetronV1(env)

			v2Env := conversion.ToV2(env, true)

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(v2Env.GetLog()))
		})
	})

	Describe("emit v2 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			msgChan := helpers.ReadFromRLP("rlp-stream-foo", false)

			env := createLogEnvelopeV2("Stream message", "rlp-stream-foo")
			helpers.EmitToMetronV2(env)

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(env.GetLog()))
		})
	})
})

func createLogEnvelopeV1(message, appID string) *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_LogMessage.Enum(),
		Origin:    proto.String(helpers.OriginName),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte(message),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appID),
		},
	}
}

func createLogEnvelopeV2(message, appID string) *v2.Envelope {
	return &v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		DeprecatedTags: map[string]*v2.Value{
			"origin": {
				Data: &v2.Value_Text{
					Text: helpers.OriginName,
				},
			},
		},
		Message: &v2.Envelope_Log{
			Log: &v2.Log{
				Payload: []byte(message),
				Type:    v2.Log_OUT,
			},
		},
	}
}
