package doppler_test

import (
	"plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ = FDescribe("GRPC Streaming Logs", func() {
	It("responds to a Stream request", func() {
		conn, err := grpc.Dial(localIPAddress+":5678", grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		client := plumbing.NewDopplerClient(conn)
		_, err = client.Stream(context.Background(), &plumbing.StreamRequest{})
		Expect(err).ToNot(HaveOccurred())
	})
})
