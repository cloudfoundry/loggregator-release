package sinkmanager_test

import (
	"doppler/sinkserver/sinkmanager"
	"plumbing"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("SinkManager GRPC", func() {
	var (
		m *sinkmanager.SinkManager
	)

	BeforeEach(func() {
		m = sinkmanager.New(
			1,
			true,
			nil,
			loggertesthelper.Logger(),
			0,
			"origin",
			time.Second,
			time.Second,
			time.Second,
			time.Second,
		)

		fakeEventEmitter.Reset()
	})

	Describe("Stream", func() {
		It("routes messages to GRPC streams", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			m.RegisterStream(&req, firstSender)

			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			m.RegisterStream(&req, secondSender)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			Eventually(firstSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
			Eventually(secondSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("doesn't send to streams for different app IDs", func() {
			req := plumbing.StreamRequest{AppID: "app"}

			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)
			m.RegisterStream(&req, sender)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("another-app", env)

			Consistently(sender.SendInput).ShouldNot(BeCalled())
		})

		It("can concurrently register Streams without data races", func() {
			req := plumbing.StreamRequest{AppID: "app"}
			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)

			go m.RegisterStream(&req, sender)

			m.RegisterStream(&req, sender)
		})

		It("continues to send while a sender is blocking", func(done Done) {
			defer close(done)

			req := plumbing.StreamRequest{AppID: "app"}

			blockingSender := newMockGRPCSender()
			m.RegisterStream(&req, blockingSender)

			workingSender := newMockGRPCSender()
			close(workingSender.SendOutput.Err)
			m.RegisterStream(&req, workingSender)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			Eventually(workingSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})
	})

	Describe("Firehose", func() {
		It("gets to drink from the firehose", func() {
			// see: https://www.youtube.com/watch?v=OXc5ltzKq3Y

			firstReq := plumbing.FirehoseRequest{SubID: "first-subscription"}
			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			m.RegisterFirehose(&firstReq, firstSender)

			secondReq := plumbing.FirehoseRequest{SubID: "second-subscription"}
			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			m.RegisterFirehose(&secondReq, secondSender)

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}
			Eventually(firstSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
			Eventually(secondSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("fans out messages to multiple senders for the same subscription", func() {
			req := plumbing.FirehoseRequest{SubID: "subscription"}

			firstSender := newMockGRPCSender()
			close(firstSender.SendOutput.Err)
			m.RegisterFirehose(&req, firstSender)

			secondSender := newMockGRPCSender()
			close(secondSender.SendOutput.Err)
			m.RegisterFirehose(&req, secondSender)

			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Origin:    proto.String("origin"),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("app"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(12.3),
					MemoryBytes:   proto.Uint64(1),
					DiskBytes:     proto.Uint64(1),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}
			var received *plumbing.Response
			select {
			case received = <-firstSender.SendInput.Resp:
			case received = <-secondSender.SendInput.Resp:
			case <-time.After(time.Second):
				Fail("Timed out waiting for firehose message")
			}
			Expect(received).To(Equal(expected))
			Consistently(firstSender.SendInput).ShouldNot(BeCalled())
			Consistently(secondSender.SendInput).ShouldNot(BeCalled())
		})

		It("can concurrently register Firehoses without data races", func() {
			req := plumbing.FirehoseRequest{SubID: "subscription"}
			sender := newMockGRPCSender()
			close(sender.SendOutput.Err)

			go m.RegisterFirehose(&req, sender)
			m.RegisterFirehose(&req, sender)
		})

		It("continues to send while a sender is blocking", func(done Done) {
			defer close(done)

			req1 := plumbing.FirehoseRequest{SubID: "sub-1"}
			req2 := plumbing.FirehoseRequest{SubID: "sub-2"}

			blockingSender := newMockGRPCSender()
			m.RegisterFirehose(&req1, blockingSender)

			workingSender := newMockGRPCSender()
			close(workingSender.SendOutput.Err)
			m.RegisterFirehose(&req2, workingSender)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			m.SendTo("app", env)

			payload, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())

			expected := &plumbing.Response{
				Payload: payload,
			}

			Eventually(workingSender.SendInput.Resp).Should(BeCalled(
				With(expected),
			))
		})

		It("reports the number of dropped messages", func() {
			req := plumbing.FirehoseRequest{SubID: "sub-1"}
			blockingSender := newMockGRPCSender()
			m.RegisterFirehose(&req, blockingSender)

			env := &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
				Origin:    proto.String("origin"),
				LogMessage: &events.LogMessage{
					Message:     []byte("I am a MESSAGE!"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}

			By("waiting for the diode to grab it's first entry")
			m.SendTo("app", env)
			Eventually(blockingSender.SendCalled).Should(Receive())

			By("over running the diode's ring buffer by 1")
			for i := 0; i < 1001; i++ {
				m.SendTo("app", env)
			}

			By("single successful end to invoke the alert")
			blockingSender.SendOutput.Err <- nil

			Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
				With("Diode.totalDroppedMessages", uint64(1000)),
			))
		})
	})
})
