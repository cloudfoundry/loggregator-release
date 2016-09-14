package doppler_test

import (
	"encoding/binary"
	"fmt"
	"net"
	"plumbing"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ = Describe("GRPC Streaming Logs", func() {
	It("responds to a Stream request", func() {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		e := &events.Envelope{
			Origin:    proto.String("foo"),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Message:     []byte("foo"),
				MessageType: events.LogMessage_OUT.Enum(),
				Timestamp:   proto.Int64(time.Now().UnixNano()),
			},
		}
		payload, err := proto.Marshal(e)
		Expect(err).ToNot(HaveOccurred())

		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		stream, err := client.Stream(ctx, &plumbing.StreamRequest{})
		Expect(err).ToNot(HaveOccurred())

		time.Sleep(time.Second)
		length := uint32(len(payload))
		prefixed := append(make([]byte, 4), payload...)
		binary.LittleEndian.PutUint32(prefixed, length)
		_, err = in.Write(prefixed)
		Expect(err).ToNot(HaveOccurred())

		msg, err := stream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(msg).ToNot(BeNil())
		Expect(msg.Payload).To(Equal(payload))
	})
})
