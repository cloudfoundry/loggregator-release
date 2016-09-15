package doppler_test

import (
	"fmt"
	"net"
	"plumbing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ = Describe("GRPC Streaming Logs", func() {
	It("responds to a Stream request", func() {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		defer in.Close()

		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		defer out.Close()
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		stream, err := client.Stream(ctx, &plumbing.StreamRequest{})
		Expect(err).ToNot(HaveOccurred())

		msg, err := stream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(msg).ToNot(BeNil())
		Expect(msg.Payload).To(Equal(streamRegisteredEvent))

		_, err = in.Write(prefixedLogMessage)
		Expect(err).ToNot(HaveOccurred())

		msg, err = stream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(msg).ToNot(BeNil())
		Expect(msg.Payload).To(Equal(logMessage))
	})

	It("responds to a Firehose request", func() {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		defer in.Close()

		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		defer out.Close()
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		firehose, err := client.Firehose(ctx, &plumbing.FirehoseRequest{})
		Expect(err).ToNot(HaveOccurred())

		msg, err := firehose.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(msg).ToNot(BeNil())
		Expect(msg.Payload).To(Equal(firehoseRegisteredEvent))

		_, err = in.Write(prefixedLogMessage)
		Expect(err).ToNot(HaveOccurred())

		msg, err = firehose.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(msg).ToNot(BeNil())
		Expect(msg.Payload).To(Equal(logMessage))
	})
})
