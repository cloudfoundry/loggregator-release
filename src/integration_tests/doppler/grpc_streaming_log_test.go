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
	var primePump = func(conn net.Conn) {
		go func() {
			for i := 0; i < 20; i++ {
				if _, err := conn.Write(prefixedPrimerMessage); err != nil {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	var waitForPrimer = func(firehose plumbing.Doppler_FirehoseClient) {
		_, err := firehose.Recv()
		Expect(err).ToNot(HaveOccurred())
	}

	var connectToDoppler = func() net.Conn {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		return in
	}

	var connectoToFirehose = func() (*grpc.ClientConn, plumbing.Doppler_FirehoseClient) {
		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		firehose, err := client.Firehose(ctx, &plumbing.FirehoseRequest{})
		Expect(err).ToNot(HaveOccurred())

		return out, firehose
	}

	var connectoToStream = func() (*grpc.ClientConn, plumbing.Doppler_StreamClient) {
		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		stream, err := client.Stream(ctx, &plumbing.StreamRequest{})
		Expect(err).ToNot(HaveOccurred())

		return out, stream
	}

	Context("with a stream connection established", func() {
		var (
			in     net.Conn
			out    *grpc.ClientConn
			stream plumbing.Doppler_StreamClient
		)

		BeforeEach(func() {
			in = connectToDoppler()
			out, stream = connectoToStream()

			primePump(in)
			waitForPrimer(stream)
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("responds to a Stream request", func() {
			_, err := in.Write(prefixedLogMessage)
			Expect(err).ToNot(HaveOccurred())

			f := func() []byte {
				msg, _ := stream.Recv()
				return msg.Payload
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})

	Context("with a firehose connection established", func() {
		var (
			in       net.Conn
			out      *grpc.ClientConn
			firehose plumbing.Doppler_FirehoseClient
		)

		BeforeEach(func() {
			in = connectToDoppler()
			out, firehose = connectoToFirehose()

			primePump(in)
			waitForPrimer(firehose)
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("responds to a Firehose request", func() {
			_, err := in.Write(prefixedLogMessage)
			Expect(err).ToNot(HaveOccurred())

			f := func() []byte {
				msg, _ := firehose.Recv()
				return msg.Payload
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
