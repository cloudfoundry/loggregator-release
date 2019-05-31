package fakes

import (
	"context"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"
	"google.golang.org/grpc"

	. "github.com/onsi/gomega"
)

func DopplerEgressV1Client(addr string) (func(), plumbing.DopplerClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	out, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return func() {
		_ = out.Close()
	}, plumbing.NewDopplerClient(out)
}

func DopplerEgressV2Client(addr string) (func(), loggregator_v2.EgressClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	out, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return func() {
		_ = out.Close()
	}, loggregator_v2.NewEgressClient(out)
}

func DopplerIngressV1Client(addr string) (func(), plumbing.DopplerIngestor_PusherClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(conn)

	var (
		pusherClient plumbing.DopplerIngestor_PusherClient
		cancel       func()
	)
	f := func() func() {
		var (
			err error
			ctx context.Context
		)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		pusherClient, err = client.Pusher(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return cancel // when the cancel func escapes the linter is happy
	}
	Eventually(f).ShouldNot(BeNil())

	return func() {
		cancel()
		conn.Close()
	}, pusherClient
}

func DopplerIngressV2Client(addr string) (func(), loggregator_v2.Ingress_SenderClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	client := loggregator_v2.NewIngressClient(conn)

	var (
		senderClient loggregator_v2.Ingress_SenderClient
		cancel       func()
	)
	f := func() func() {
		var (
			err error
			ctx context.Context
		)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		senderClient, err = client.Sender(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return cancel // when the cancel func escapes the linter is happy
	}
	Eventually(f).ShouldNot(BeNil())

	return func() {
		cancel()
		conn.Close()
	}, senderClient
}
