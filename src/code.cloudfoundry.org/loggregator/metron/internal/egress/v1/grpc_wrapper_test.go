package v1_test

import (
	"errors"

	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCWrapper", func() {
	var (
		envelope    *events.Envelope
		grpcWrapper *egress.GRPCWrapper
		message     []byte

		mockBatcher *mockMetricBatcher
		mockConn    *mockConn
	)

	BeforeEach(func() {
		mockBatcher = newMockMetricBatcher()
		metrics.Initialize(nil, mockBatcher)

		mockConn = newMockConn()

		envelope = &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		grpcWrapper = egress.NewGRPCWrapper(mockConn)

		var err error
		message, err = proto.Marshal(envelope)
		Expect(err).NotTo(HaveOccurred())
	})

	It("counts the number of bytes sent", func() {
		// Make sure the counter counts the sent byte count,
		// instead of len(message)
		sentLength := len(message)
		mockConn.WriteOutput.Ret0 <- nil

		err := grpcWrapper.Write(message)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
			With("grpc.sentByteCount", uint64(sentLength)),
		))
	})

	It("counts the number of messages sent", func() {
		mockConn.WriteOutput.Ret0 <- nil
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		err := grpcWrapper.Write(message, mockChainer)
		Expect(err).NotTo(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("grpc.sentMessageCount"),
		))
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("DopplerForwarder.sentMessages"),
		))
		Eventually(mockChainer.SetTagInput).Should(BeCalled(
			With("protocol", "grpc"),
		))
		Eventually(mockChainer.IncrementCalled).Should(BeCalled())
	})

	It("increments transmitErrorCount *only* if client write fails", func() {
		mockConn.WriteOutput.Ret0 <- nil

		err := grpcWrapper.Write(message)
		Expect(err).NotTo(HaveOccurred())
		Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled(
			With("grpc.sendErrorCount"),
		))

		err = errors.New("Client Write Failed")
		mockConn.WriteOutput.Ret0 <- err

		err = grpcWrapper.Write(message)
		Expect(err).To(HaveOccurred())
		Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
			With("grpc.sendErrorCount"),
		))
	})

	Context("when client write fail", func() {
		BeforeEach(func() {
			mockConn.WriteOutput.Ret0 <- errors.New("failed")
		})

		It("returns an error", func() {
			err := grpcWrapper.Write(message)
			Expect(err).To(HaveOccurred())
		})

		It("does not increment message count or sentMessages", func() {
			grpcWrapper.Write(message)

			var name string
			Eventually(mockBatcher.BatchIncrementCounterInput.Name).Should(Receive(&name))
			Expect(name).To(Equal("grpc.sendErrorCount"))
			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())

			Consistently(mockBatcher.BatchIncrementCounterInput).ShouldNot(BeCalled())
		})
	})
})
