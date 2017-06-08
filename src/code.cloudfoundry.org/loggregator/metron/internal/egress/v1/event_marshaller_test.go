package v1_test

import (
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventMarshaller", func() {
	var (
		marshaller      *egress.EventMarshaller
		mockBatcher     *mockEventBatcher
		mockChainWriter *mockBatchChainByteWriter
		mockChainer     *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockBatcher = newMockEventBatcher()
		mockChainWriter = newMockBatchChainByteWriter()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)
	})

	JustBeforeEach(func() {
		marshaller = egress.NewMarshaller(mockBatcher)
		marshaller.SetWriter(mockChainWriter)
	})

	Describe("Write", func() {
		var envelope *events.Envelope

		Context("with a nil writer", func() {
			BeforeEach(func() {
				envelope = &events.Envelope{
					Origin:    proto.String("The Negative Zone"),
					EventType: events.Envelope_LogMessage.Enum(),
				}
			})

			JustBeforeEach(func() {
				marshaller.SetWriter(nil)
			})

			It("does not panic", func() {
				Expect(func() {
					marshaller.Write(envelope)
				}).ToNot(Panic())
			})
		})

		Context("with an invalid envelope", func() {
			BeforeEach(func() {
				envelope = &events.Envelope{}
				close(mockChainWriter.WriteOutput.Err)
			})

			It("counts marshal errors", func() {
				marshaller.Write(envelope)
				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("dropsondeMarshaller.marshalErrors"),
				))
			})

			It("doesn't write the bytes", func() {
				marshaller.Write(envelope)
				Consistently(mockChainWriter.WriteCalled).ShouldNot(Receive())
			})
		})

		Context("with writer", func() {
			BeforeEach(func() {
				close(mockChainWriter.WriteOutput.Err)
				envelope = &events.Envelope{
					Origin:    proto.String("The Negative Zone"),
					EventType: events.Envelope_LogMessage.Enum(),
				}
			})

			It("writes messages to the writer", func() {
				marshaller.Write(envelope)
				expected, err := proto.Marshal(envelope)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
					With("dropsondeMarshaller.sentEnvelopes"),
				))
				Eventually(mockChainer.SetTagInput).Should(BeCalled(
					With("event_type", "LogMessage"),
				))
				Eventually(mockChainWriter.WriteInput).Should(BeCalled(
					With(expected, []metricbatcher.BatchCounterChainer{mockChainer}),
				))
			})
		})
	})

	Describe("SetWriter", func() {
		It("writes to the new writer", func() {
			newWriter := newMockBatchChainByteWriter()
			close(newWriter.WriteOutput.Err)
			marshaller.SetWriter(newWriter)

			envelope := &events.Envelope{
				Origin:    proto.String("The Negative Zone"),
				EventType: events.Envelope_LogMessage.Enum(),
			}
			marshaller.Write(envelope)

			expected, err := proto.Marshal(envelope)
			Expect(err).ToNot(HaveOccurred())
			Consistently(mockChainWriter.WriteInput).ShouldNot(BeCalled())
			Eventually(newWriter.WriteInput).Should(BeCalled(With(expected)))
		})
	})
})
