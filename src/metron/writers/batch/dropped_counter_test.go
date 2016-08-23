package batch_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"metron/config"
	"metron/writers/batch"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const origin = "test-origin"

var _ = Describe("DroppedCounter", func() {
	It("doesn't write when the dropped count is zero", func() {
		counter, byteWriter := newCounterAndMockWriter()
		counter.Drop(0)

		Consistently(byteWriter.WriteInput).ShouldNot(BeCalled())
	})

	It("records dropped messages", func() {
		counter, _ := newCounterAndMockWriter()
		counter.Drop(10)
		Expect(counter.Dropped()).To(BeEquivalentTo(10))
	})

	It("sends messages about the number of messages dropped", func() {
		counter, byteWriter := newCounterAndMockWriter()
		counter.Drop(10)

		var messages []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&messages))

		actualEnvelopes, err := unmarshalMessages(messages)
		Expect(err).ToNot(HaveOccurred())
		actualCounter := actualEnvelopes[0]

		Expect(actualCounter.GetOrigin()).To(Equal(origin))
		Expect(actualCounter.EventType).To(Equal(events.Envelope_CounterEvent.Enum()))
		Expect(actualCounter.CounterEvent.GetName()).To(ContainSubstring("DroppedCounter.droppedMessageCount"))
		Expect(actualCounter.CounterEvent.GetDelta()).To(BeNumerically("==", 10))
		Expect(actualCounter.GetTimestamp()).ToNot(BeZero())
		Expect(actualCounter.GetIp()).ToNot(BeEmpty())
		Expect(actualCounter.GetDeployment()).ToNot(BeEmpty())
		Expect(actualCounter.GetJob()).ToNot(BeEmpty())
		Expect(actualCounter.GetIndex()).ToNot(BeEmpty())

		actualLog := actualEnvelopes[1]
		Expect(actualLog.GetOrigin()).To(Equal(origin))
		Expect(actualLog.EventType).To(Equal(events.Envelope_LogMessage.Enum()))
		Expect(actualLog.LogMessage.Message).To(ContainSubstring("Dropped 10 message(s) from MetronAgent to Doppler"))
		Expect(actualLog.LogMessage.GetSourceType()).To(Equal("MET"))
		Expect(actualLog.GetTimestamp()).ToNot(BeZero())
		Expect(actualLog.LogMessage.GetTimestamp()).ToNot(BeZero())
		Expect(actualLog.GetIp()).ToNot(BeEmpty())
		Expect(actualLog.GetDeployment()).ToNot(BeEmpty())
		Expect(actualLog.GetJob()).ToNot(BeEmpty())
		Expect(actualLog.GetIndex()).ToNot(BeEmpty())
	})

	It("resets the counter after it sends the dropped message count message", func() {
		counter, byteWriter := newCounterAndMockWriter()
		counter.Drop(10)

		var sent []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&sent))

		byteWriter.WriteOutput.SentLength <- len(sent)
		byteWriter.WriteOutput.Err <- nil

		Eventually(counter.Dropped).Should(BeZero())
	})

	It("maintains the total as a sum", func() {
		counter, byteWriter := newCounterAndMockWriter()
		counter.Drop(10)
		var sent []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&sent))

		byteWriter.WriteOutput.SentLength <- len(sent)
		byteWriter.WriteOutput.Err <- nil

		counter.Drop(10)
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&sent))

		actualEnvelopes, err := unmarshalMessages(sent)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualEnvelopes).To(HaveLen(2))

		Expect(actualEnvelopes[0].CounterEvent.GetTotal()).To(BeNumerically("==", 20))
		Expect(actualEnvelopes[0].CounterEvent.GetDelta()).To(BeNumerically("==", 10))
	})

	It("does not overwrite dropped counts added while flushing a message", func() {
		By("queuing up a message about previously dropped messages")
		counter, byteWriter := newCounterAndMockWriter()
		counter.Drop(10)

		var sent []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&sent))

		By("adding another message to the queue while flushing the previous message is blocked")
		counter.Drop(5)
		Consistently(byteWriter.WriteInput).ShouldNot(BeCalled())

		By("flushing the previous message")
		byteWriter.WriteOutput.SentLength <- len(sent)
		byteWriter.WriteOutput.Err <- nil

		By("sending a second message about the 5 messages which have been dropped while waiting for flushing")
		Eventually(counter.Dropped).Should(BeNumerically("==", 5))

		var messages []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&messages))

		actualEnvelopes, err := unmarshalMessages(messages)
		Expect(err).ToNot(HaveOccurred())
		actualCounter := actualEnvelopes[0]

		Expect(actualCounter.GetOrigin()).To(Equal(origin))
		Expect(actualCounter.EventType).To(Equal(events.Envelope_CounterEvent.Enum()))
		Expect(actualCounter.CounterEvent.GetName()).To(ContainSubstring("DroppedCounter.droppedMessageCount"))
		Expect(actualCounter.CounterEvent.GetDelta()).To(BeNumerically("==", 5))

		actualLog := actualEnvelopes[1]
		Expect(actualLog.GetOrigin()).To(Equal(origin))
		Expect(actualLog.EventType).To(Equal(events.Envelope_LogMessage.Enum()))
		Expect(actualLog.LogMessage.Message).To(ContainSubstring("Dropped 5 message(s) from MetronAgent to Doppler"))
	})

	It("retries sending messages when sending errors", func() {
		incrementer := newMockBatchCounterIncrementer()
		byteWriter := newMockBatchChainByteWriter()
		counter := batch.NewDroppedCounter(byteWriter, incrementer, origin, "some-ip", new(config.Config))

		byteWriter.WriteOutput.SentLength <- 0
		byteWriter.WriteOutput.Err <- errors.New("boom")

		counter.Drop(20)

		Eventually(incrementer.BatchIncrementCounterInput).Should(BeCalled(
			With("droppedCounter.sendErrors"),
		))
		Expect(byteWriter.WriteInput.Message).To(Receive())

		var messages []byte
		Eventually(byteWriter.WriteInput.Message).Should(Receive(&messages))

		actualEnvelopes, err := unmarshalMessages(messages)
		Expect(err).ToNot(HaveOccurred())
		actualCounter := actualEnvelopes[0]

		Expect(actualCounter.GetOrigin()).To(Equal(origin))
		Expect(actualCounter.EventType).To(Equal(events.Envelope_CounterEvent.Enum()))
		Expect(actualCounter.CounterEvent.GetName()).To(ContainSubstring("DroppedCounter.droppedMessageCount"))
		Expect(actualCounter.CounterEvent.GetDelta()).To(BeNumerically("==", 20))

		actualLog := actualEnvelopes[1]
		Expect(actualLog.GetOrigin()).To(Equal(origin))
		Expect(actualLog.EventType).To(Equal(events.Envelope_LogMessage.Enum()))
		Expect(actualLog.LogMessage.Message).To(ContainSubstring("Dropped 20 message(s) from MetronAgent to Doppler"))
	})
})

func newCounterAndMockWriter() (*batch.DroppedCounter, *mockBatchChainByteWriter) {
	byteWriter := newMockBatchChainByteWriter()
	conf := &config.Config{
		Deployment: "some-deployment",
		Job:        "some-job",
		Index:      "some-index",
	}
	counter := batch.NewDroppedCounter(byteWriter, newMockBatchCounterIncrementer(), origin, "some-ip", conf)

	return counter, byteWriter
}

func unmarshalMessages(messages []byte) ([]*events.Envelope, error) {
	buffer := bytes.NewBuffer(messages)

	var envelopes []*events.Envelope
	for {
		var size uint32
		err := binary.Read(buffer, binary.LittleEndian, &size)
		if err == io.EOF {
			return envelopes, nil
		}
		if err != nil {
			return nil, err
		}

		msgBytes := make([]byte, size)
		if _, err := buffer.Read(msgBytes); err != nil {
			return nil, err
		}

		message := events.Envelope{}
		if err := proto.Unmarshal(msgBytes, &message); err != nil {
			return nil, err
		}
		envelopes = append(envelopes, &message)
	}
}
