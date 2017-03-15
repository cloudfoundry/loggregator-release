package doppler_test

import (
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/cloudfoundry/dropsonde/signature"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/config"
	"plumbing"
)

var _ = Describe("ContainerMetrics", func() {
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

		It("gets container metrics", func() {
			containerMetric := buildContainerMetric()
			signedMetric := signature.SignMessage(containerMetric, []byte("secret"))

			_, err := in.Write(signedMetric)
			Expect(err).ToNot(HaveOccurred())

			ctx := context.Background()
			req := &plumbing.ContainerMetricsRequest{AppID: "test-app"}

			f := func() []byte {
				msg, _ := client.ContainerMetrics(ctx, req)
				if msg == nil || len(msg.Payload) == 0 {
					return nil
				}
				return msg.Payload[0]
			}
			Eventually(f).Should(Equal(containerMetric))
		})
	})
})
