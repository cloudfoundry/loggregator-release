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

	var waitForPrimer = func(subscription plumbing.Doppler_SubscribeClient) {
		_, err := subscription.Recv()
		Expect(err).ToNot(HaveOccurred())
	}

	var connectToDoppler = func() net.Conn {
		in, err := net.Dial("tcp", fmt.Sprintf(localIPAddress+":4321"))
		Expect(err).ToNot(HaveOccurred())
		return in
	}

	var connectoToSubscription = func(req plumbing.SubscriptionRequest) (*grpc.ClientConn, plumbing.Doppler_SubscribeClient) {
		out, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		client := plumbing.NewDopplerClient(out)
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		subscription, err := client.Subscribe(ctx, &req)
		Expect(err).ToNot(HaveOccurred())

		return out, subscription
	}

	Context("with a subscription established", func() {
		var (
			in           net.Conn
			out          *grpc.ClientConn
			subscription plumbing.Doppler_SubscribeClient
		)

		BeforeEach(func() {
			in = connectToDoppler()
			out, subscription = connectoToSubscription(
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
				return msg.Payload
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
