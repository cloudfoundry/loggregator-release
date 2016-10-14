package doppler_test

import (
	"fmt"
	"net"
	"plumbing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecentLogs", func() {
	var connectToDoppler = func() net.Conn {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		return in
	}

	var connectToGRPC = func() (*grpc.ClientConn, plumbing.DopplerClient) {
		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		return out, plumbing.NewDopplerClient(out)
	}

	Context("with a client connected to GRPC", func() {
		var (
			in     net.Conn
			out    *grpc.ClientConn
			client plumbing.DopplerClient
		)

		BeforeEach(func() {
			in = connectToDoppler()
			out, client = connectToGRPC()
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("gets recent logs", func() {
			_, err := in.Write(prefixedLogMessage)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			ctx := context.Background()
			req := &plumbing.RecentLogsRequest{
				AppID: "test-app",
			}
			f := func() []byte {
				msg, _ := client.RecentLogs(ctx, req)
				if len(msg.Payload) == 0 {
					return nil
				}
				return msg.Payload[0]
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
