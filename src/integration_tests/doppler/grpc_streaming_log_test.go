package doppler_test

import (
	"doppler/config"
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

	var waitForPrimer = func(subscription plumbing.Doppler_SubscribeClient) {
		_, err := subscription.Recv()
		Expect(err).ToNot(HaveOccurred())
	}

	var connectToDoppler = func() net.Conn {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		return in
	}

	var connectoToSubscription = func(conf *config.Config, req plumbing.SubscriptionRequest) (*grpc.ClientConn, plumbing.Doppler_SubscribeClient) {
		conn, client := connectToGRPC(conf)

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		subscription, err := client.Subscribe(ctx, &req)
		Expect(err).ToNot(HaveOccurred())

		return conn, subscription
	}

	Context("with a subscription established", func() {
		var (
			conf         *config.Config
			in           net.Conn
			out          *grpc.ClientConn
			subscription plumbing.Doppler_SubscribeClient
		)

		BeforeEach(func() {
			conf = fetchDopplerConfig("fixtures/doppler.json")
			in = connectToDoppler()
			out, subscription = connectoToSubscription(
				conf,
				plumbing.SubscriptionRequest{
					ShardID: "foo",
					Filter: &plumbing.Filter{
						AppID: "test-app",
					},
				},
			)

			primePump(in)
			waitForPrimer(subscription)
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("responds to a subscription request", func() {
			_, err := in.Write(prefixedLogMessage)
			Expect(err).ToNot(HaveOccurred())

			f := func() []byte {
				msg, _ := subscription.Recv()
				if msg == nil {
					return nil
				}

				return msg.Payload
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
