package messagewriter_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"
	"tools/metronbenchmark/messagewriter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessageWriter", func() {
	It("should send a message to the writer with the round ID and timestamp", func() {
		b := &bytes.Buffer{}
		now := time.Now()
		roundId := uint(0)
		msgWriter := messagewriter.NewMessageWriter(b)
		msgWriter.Send(roundId, now)
		data, err := ioutil.ReadAll(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(BeEquivalentTo(formatMsgString(0, 1, now)))
	})

	It("should send a message to the writer with the sequence", func() {
		b := &bytes.Buffer{}
		now := time.Now()
		roundId := uint(0)
		msgWriter := messagewriter.NewMessageWriter(b)
		msgWriter.Send(roundId, now)
		msgWriter.Send(roundId, now)
		data, err := ioutil.ReadAll(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(ContainSubstring(formatMsgString(0, 1, now)))
		Expect(data).To(ContainSubstring(formatMsgString(0, 2, now)))
	})

	It("should keep track of the total sent messages", func() {
		b := &bytes.Buffer{}
		now := time.Now()
		roundId := uint(0)
		msgWriter := messagewriter.NewMessageWriter(b)
		msgWriter.Send(roundId, now)
		msgWriter.Send(roundId, now)

		Expect(msgWriter.GetSentCount(roundId)).To(BeEquivalentTo(2))
	})

	It("should keep track of different rounds' sequences independently", func() {
		b := &bytes.Buffer{}
		now := time.Now()
		roundId1 := uint(1)
		roundId2 := uint(2)
		msgWriter := messagewriter.NewMessageWriter(b)
		msgWriter.Send(roundId1, now)
		msgWriter.Send(roundId1, now)

		msgWriter.Send(roundId2, now)

		data, err := ioutil.ReadAll(b)
		Expect(err).NotTo(HaveOccurred())

		Expect(data).To(ContainSubstring(formatMsgString(1, 1, now)))
		Expect(data).To(ContainSubstring(formatMsgString(1, 2, now)))

		Expect(data).To(ContainSubstring(formatMsgString(2, 1, now)))
	})

	It("should return empty result for a non-existent round", func() {
		b := &bytes.Buffer{}
		roundId := uint(1)
		now := time.Now()
		msgWriter := messagewriter.NewMessageWriter(b)
		msgWriter.Send(roundId, now)

		data, err := ioutil.ReadAll(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(ContainSubstring(formatMsgString(1, 1, now)))

		Expect(msgWriter.GetSentCount(roundId)).To(BeEquivalentTo(1))
		Expect(msgWriter.GetSentCount(2)).To(BeZero())
	})
})

func formatMsgString(roundId int, sequence int, t time.Time) string {
	return fmt.Sprintf("msg:%d:%d:%d:\n", roundId, sequence, t.UnixNano())
}
