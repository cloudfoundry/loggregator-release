package doppler_test

import (
	"doppler/config"
	"net"
	"plumbing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/dropsonde/signature"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecentLogs", func() {
	var connectToDoppler = func() net.Conn {
		in, err := net.Dial("udp", localIPAddress+":8765")
		Expect(err).ToNot(HaveOccurred())
		return in
	}

	Context("with a client connected to GRPC", func() {
		var (
			conf   *config.Config
			in     net.Conn
			out    *grpc.ClientConn
			client plumbing.DopplerClient
		)

		BeforeEach(func() {
			conf = fetchDopplerConfig("fixtures/doppler.json")
			in = connectToDoppler()
			out, client = connectToGRPC(conf)
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("gets recent logs", func() {
			logMessage := buildLogMessage()
			signedLog := signature.SignMessage(logMessage, []byte("secret"))

			_, err := in.Write(signedLog)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			ctx := context.Background()
			req := &plumbing.RecentLogsRequest{
				AppID: "test-app",
			}
			f := func() []byte {
				msg, _ := client.RecentLogs(ctx, req)
				if msg == nil || len(msg.Payload) == 0 {
					return nil
				}
				return msg.Payload[0]
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
