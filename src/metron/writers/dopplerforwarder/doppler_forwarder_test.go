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
	"truncatingbuffer"
)

var sharedSecret = []byte("secret")

var _ = Describe("DopplerForwarder", func() {
	var (
		sender           *fake.FakeMetricSender
		clientPool       *fakes.FakeClientPool
		client           *fakeclient.FakeClient
		logger           *gosteno.Logger
		forwarder        *dopplerforwarder.DopplerForwarder
		envelope         *events.Envelope
		inputChannel     chan *events.Envelope
		stopChan         chan struct{}
		truncatingBuffer *truncatingbuffer.TruncatingBuffer
		doneChan chan struct {}
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		client = &fakeclient.FakeClient{}
		clientPool = &fakes.FakeClientPool{}
		clientPool.RandomClientReturns(client, nil)

		logger = loggertesthelper.Logger()
		loggertesthelper.TestLoggerSink.Clear()

		context := truncatingbuffer.NewDefaultContext("origin", "id")
		inputChannel = make(chan *events.Envelope)
		stopChan = make(chan struct{})
		truncatingBuffer = truncatingbuffer.NewTruncatingBuffer(inputChannel, 100, context, loggertesthelper.Logger(), stopChan)
		forwarder = dopplerforwarder.New(clientPool, sharedSecret, truncatingBuffer, logger)
		doneChan = make(chan struct {})
		go func(){
			forwarder.Run()
			doneChan <- struct {}{}
		}()
		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
	})

	AfterEach(func() {
		<- doneChan
		close(stopChan)
	})

	Context("With TruncatingBuffer", func() {
		BeforeEach(func() {
			client.SchemeReturns("udp")
		})
		It("Should read from truncatingBuffer", func() {
			inputChannel <- envelope
			Eventually(client.WriteCallCount).Should(Equal(1))
			//Expect(client.WriteArgsForCall(0)).To(Equal())
		})
	})

	Context("client selection", func() {
		It("selects a random client", func() {
			inputChannel <- envelope
			Eventually(func() int{return clientPool.RandomClientCallCount()}).Should(Equal(1))
		})

		Context("when selecting a client errors", func() {
			It("an error is logged and returns", func() {
				clientPool.RandomClientReturns(nil, errors.New("boom"))
				inputChannel <- envelope

				Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("can't forward message"))
				Eventually(client.SchemeCallCount).Should(Equal(0))
			})
		})
	})

	Context("udp client", func() {
		BeforeEach(func() {
			client.SchemeReturns("udp")
		})

		It("counts the number of bytes sent", func() {
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())

			client.WriteReturns(len(bytes), nil)

			n := uint32(len(bytes))

			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("udp.sentByteCount")
			}).Should(BeEquivalentTo(n))
		})

		It("counts the number of messages sent", func() {
			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("udp.sentMessageCount")
			}).Should(BeEquivalentTo(1))
		})

		It("increments transmitErrorCount if client write fails", func() {
			err := errors.New("Client Write Failed")
			client.WriteReturns(0, err)

			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("udp.sendErrorCount")
			}).Should(BeEquivalentTo(1))
		})

		It("marshals, signs, writes and emits metrics", func() {
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())
			bytes = signature.SignMessage(bytes, sharedSecret)

			inputChannel <- envelope

			Eventually(client.WriteCallCount).Should(Equal(1))
			Eventually(func() []byte { return client.WriteArgsForCall(0) }).Should(Equal(bytes))
			Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.logMessageMarshalled") }).Should(BeEquivalentTo(1))
		})

		Context("when writes fail", func() {
			BeforeEach(func() {
				client.WriteReturns(0, errors.New("boom"))
			})

			It("does not increment message count or sentMessages", func() {
				inputChannel <- envelope

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.logMessageMarshalled") }).Should(BeZero())
			})
		})
	})

	Context("tls client", func() {
		BeforeEach(func() {
			client.SchemeReturns("tls")
		})

		It("counts the number of bytes sent", func() {
			var buffer bytes.Buffer
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())

			client.WriteReturns(len(bytes), nil)

			n := uint32(len(bytes))
			err = binary.Write(&buffer, binary.LittleEndian, n)
			Expect(err).NotTo(HaveOccurred())

			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("tls.sentByteCount")
			}).Should(BeEquivalentTo(n + 4))
		})

		It("counts the number of messages sent", func() {
			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("tls.sentMessageCount")
			}).Should(BeEquivalentTo(1))
		})

		It("increments transmitErrorCount if client write fails", func() {
			err := errors.New("Client Write Failed")
			client.WriteReturns(0, err)

			inputChannel <- envelope

			Eventually(func() uint64 {
				return sender.GetCounter("tls.sendErrorCount")
			}).Should(BeEquivalentTo(1))
		})

		It("stream and emits metrics", func() {
			var buffer bytes.Buffer
			bytes, err := proto.Marshal(envelope)
			Expect(err).NotTo(HaveOccurred())

			n := uint32(len(bytes))
			err = binary.Write(&buffer, binary.LittleEndian, n)
			Expect(err).NotTo(HaveOccurred())

			inputChannel <- envelope

			Eventually(func() int { return client.WriteCallCount() }).Should(Equal(2))
			Eventually(func() []byte { return client.WriteArgsForCall(0) }).Should(Equal(buffer.Bytes()))
			Eventually(func() []byte { return client.WriteArgsForCall(1) }).Should(Equal(bytes))

			Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
			Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.logMessageMarshalled") }).Should(BeEquivalentTo(1))
		})

		Context("with a network error", func() {
			BeforeEach(func() {
				client.WriteReturns(0, &net.OpError{Op: "dial", Err: errors.New("boom")})

				inputChannel <- envelope
			})

			It("closes the client", func() {
				Eventually(func() int { return client.CloseCallCount() }).Should(Equal(1))
			})

			It("increments the marshallErrors metric", func() {
				Consistently(func() uint64 { return sender.GetCounter("dropsondeMarshaller.marshalErrors") }).Should(BeZero())
			})

			It("does not increment message count or sentMessages", func() {
				inputChannel <- envelope

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.LogMessageMarshalled") }).Should(BeZero())
			})
		})

		Context("with a non-network error", func() {
			BeforeEach(func() {
				client.WriteReturns(0, errors.New("boom"))

				inputChannel <- envelope
			})

			It("closes the client", func() {
				Eventually(func() int { return client.CloseCallCount() }).Should(Equal(1))
			})

			It("does not increment message count or sentMessages", func() {
				inputChannel <- envelope

				Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
				Eventually(func() uint64 { return sender.GetCounter("dropsondeMarshaller.LogMessageMarshalled") }).Should(BeZero())
			})
		})
	})

	Context("unknown scheme", func() {
		BeforeEach(func() {
			client.SchemeReturns("unknown")
		})

		It("logs an error and returns", func() {
			inputChannel <- envelope

			Eventually(func() string{return loggertesthelper.TestLoggerSink.LogContents()}).Should(ContainSubstring("unknown protocol"))
			Consistently(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeZero())
		})
	})
})
