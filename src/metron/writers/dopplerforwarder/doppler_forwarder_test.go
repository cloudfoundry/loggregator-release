package dopplerforwarder_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"metron/clientpool/fakeclient"
	"metron/writers/dopplerforwarder"
	"metron/writers/dopplerforwarder/fakes"
	"net"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var sharedSecret = []byte("secret")

var _ = Describe("DopplerForwarder", func() {
	var (
		sender     *fake.FakeMetricSender
		clientPool *fakes.FakeClientPool
		client     *fakeclient.FakeClient
		logger     *gosteno.Logger
		forwarder  *dopplerforwarder.DopplerForwarder
		envelope   *events.Envelope
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		client = &fakeclient.FakeClient{}
		clientPool = &fakes.FakeClientPool{}
		clientPool.RandomClientReturns(client, nil)

		logger = loggertesthelper.Logger()
		loggertesthelper.TestLoggerSink.Clear()

		forwarder = dopplerforwarder.New(clientPool, sharedSecret, logger)

		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
	})

	Context("client selection", func() {
		It("selects a random client", func() {
			forwarder.Write(envelope)
			Expect(clientPool.RandomClientCallCount()).To(Equal(1))
		})

		Context("when selecting a client errors", func() {
			It("an error is logged and returns", func() {
				clientPool.RandomClientReturns(nil, errors.New("boom"))
				forwarder.Write(envelope)

				Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("can't forward message"))
				Expect(client.SchemeCallCount()).To(Equal(0))
			})
		})
	})

	Context("udp client", func() {
		BeforeEach(func() {
			client.SchemeReturns("udp")
		})

		It("marshals, signs, writes and emits metrics", func() {
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())
			bytes = signature.SignMessage(bytes, sharedSecret)

			forwarder.Write(envelope)

			Expect(client.WriteArgsForCall(0)).To(Equal(bytes))
			Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			Expect(sender.GetCounter("dropsondeMarshaller.logMessageMarshalled")).To(BeEquivalentTo(1))
		})

		Context("when writes fail", func() {
			BeforeEach(func() {
				client.WriteReturns(0, errors.New("boom"))
			})

			It("does not increment message count or sentMessages", func() {
				forwarder.Write(envelope)

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Expect(sender.GetCounter("dropsondeMarshaller.LogMessageMarshalled")).To(BeZero())
			})
		})
	})

	Context("tls client", func() {
		BeforeEach(func() {
			client.SchemeReturns("tls")
		})

		It("stream and emits metrics", func() {
			var buffer bytes.Buffer
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())

			n := uint32(len(bytes))
			err = binary.Write(&buffer, binary.LittleEndian, n)
			Expect(err).NotTo(HaveOccurred())

			forwarder.Write(envelope)

			Expect(client.WriteCallCount()).To(Equal(2))
			Expect(client.WriteArgsForCall(0)).To(Equal(buffer.Bytes()))
			Expect(client.WriteArgsForCall(1)).To(Equal(bytes))

			Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			Expect(sender.GetCounter("dropsondeMarshaller.logMessageMarshalled")).To(BeEquivalentTo(1))
		})

		Context("with a network error", func() {
			BeforeEach(func() {
				client.WriteReturns(0, &net.OpError{Op: "dial", Err: errors.New("boom")})

				forwarder.Write(envelope)
			})

			It("closes the client", func() {
				Expect(client.CloseCallCount()).To(Equal(1))
			})

			It("increments the marshallErrors metric", func() {
				Consistently(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeZero())
			})

			It("does not increment message count or sentMessages", func() {
				forwarder.Write(envelope)

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Expect(sender.GetCounter("dropsondeMarshaller.LogMessageMarshalled")).To(BeZero())
			})
		})

		Context("with a non-network error", func() {
			BeforeEach(func() {
				client.WriteReturns(0, errors.New("boom"))

				forwarder.Write(envelope)
			})

			It("closes the client", func() {
				Expect(client.CloseCallCount()).To(Equal(1))
			})

			It("does not increment message count or sentMessages", func() {
				forwarder.Write(envelope)

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Expect(sender.GetCounter("dropsondeMarshaller.LogMessageMarshalled")).To(BeZero())
			})
		})
	})

	Context("unknown scheme", func() {
		BeforeEach(func() {
			client.SchemeReturns("unknown")
		})

		It("logs an error and returns", func() {
			forwarder.Write(envelope)

			Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("unknown protocol"))
			Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
		})
	})
})
