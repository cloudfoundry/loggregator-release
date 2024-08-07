package metricemitter_test

import (
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emitter Client", func() {
	It("maintains a gRPC connection", func() {
		grpcServer := newgRPCServer()
		defer grpcServer.stop()

		_, err := metricemitter.NewClient(
			grpcServer.addr,
			metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			metricemitter.WithPulseInterval(50*time.Millisecond),
		)

		Expect(err).ToNot(HaveOccurred())
	})

	It("reconnects if the connection is lost", func() {
		grpcServer := newgRPCServer()

		client, err := metricemitter.NewClient(
			grpcServer.addr,
			metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			metricemitter.WithPulseInterval(50*time.Millisecond),
		)
		Expect(err).ToNot(HaveOccurred())

		client.NewGauge("some-name", "some-unit")
		Eventually(grpcServer.senders, 2).Should(HaveLen(1))
		Eventually(func() int {
			return len(grpcServer.envelopes)
		}).Should(BeNumerically(">=", 1))

		grpcServer.stop()

		envelopeCount := len(grpcServer.envelopes)
		Consistently(grpcServer.envelopes).Should(HaveLen(envelopeCount))

		grpcServer = newgRPCServerWithAddr(grpcServer.addr)
		defer grpcServer.stop()

		Eventually(func() int {
			return len(grpcServer.envelopes)
		}, 3).Should(BeNumerically(">", envelopeCount))
	})

	It("creates a new metric", func() {
		grpcServer := newgRPCServer()
		defer grpcServer.stop()

		client, err := metricemitter.NewClient(
			grpcServer.addr,
			metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			metricemitter.WithPulseInterval(50*time.Millisecond),
		)
		Expect(err).ToNot(HaveOccurred())

		client.NewCounter("some-name")
		Eventually(grpcServer.senders).Should(HaveLen(1))
	})

	It("emits an event", func() {
		grpcServer := newgRPCServer()
		defer grpcServer.stop()

		client, err := metricemitter.NewClient(
			grpcServer.addr,
			metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			metricemitter.WithPulseInterval(50*time.Millisecond),
			metricemitter.WithSourceID("some-id"),
		)
		Expect(err).ToNot(HaveOccurred())

		client.EmitEvent("some-title", "some-body")

		var e *loggregator_v2.Envelope
		Eventually(grpcServer.envelopes).Should(Receive(&e))
		Expect(e.GetEvent().GetTitle()).To(Equal("some-title"))
		Expect(e.GetEvent().GetBody()).To(Equal("some-body"))
		Expect(e.GetTimestamp()).To(BeNumerically("~", time.Now().UnixNano(), time.Second))
		Expect(e.GetSourceId()).To(Equal("some-id"))
	})

	It("does not try to emit an event when loggregator is not available", func() {
		client, err := metricemitter.NewClient(
			"127.0.0.1:9093",
			metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			metricemitter.WithPulseInterval(50*time.Millisecond),
		)
		Expect(err).ToNot(HaveOccurred())

		f := func() {
			client.EmitEvent("some-title", "some-body")
		}
		Expect(f).ToNot(Panic())
	})

	Describe("synchronous data frame emission", func() {
		It("emits a zero value on an interval", func() {
			grpcServer := newgRPCServer()
			defer grpcServer.stop()

			client, err := metricemitter.NewClient(
				grpcServer.addr,
				metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
				metricemitter.WithPulseInterval(50*time.Millisecond),
				metricemitter.WithSourceID("a-source"),
			)
			Expect(err).ToNot(HaveOccurred())

			client.NewCounter("some-name")
			Eventually(grpcServer.senders).Should(HaveLen(1))

			var env *loggregator_v2.Envelope
			Consistently(func() uint64 {
				Eventually(grpcServer.envelopes).Should(Receive(&env))
				Expect(env.SourceId).To(Equal("a-source"))
				Expect(env.Timestamp).To(BeNumerically(">", 0))

				counter := env.GetCounter()
				Expect(counter.Name).To(Equal("some-name"))

				return env.GetCounter().GetDelta()
			}).Should(Equal(uint64(0)))
		})

		It("always combines the tags from the client and the metric", func() {
			grpcServer := newgRPCServer()
			defer grpcServer.stop()

			client, err := metricemitter.NewClient(
				grpcServer.addr,
				metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
				metricemitter.WithPulseInterval(50*time.Millisecond),
				metricemitter.WithSourceID("a-source"),
				metricemitter.WithOrigin("a-origin"),
				metricemitter.WithDeployment("a-deployment", "a-job", "a-index"),
			)
			Expect(err).ToNot(HaveOccurred())

			client.NewCounter("some-name",
				metricemitter.WithVersion(2, 0),
				metricemitter.WithTags(map[string]string{
					"unicorn": "another-unicorn",
				}),
			)
			Eventually(grpcServer.senders).Should(HaveLen(1))

			var env *loggregator_v2.Envelope
			text := func(s string) *loggregator_v2.Value {
				return &loggregator_v2.Value{Data: &loggregator_v2.Value_Text{Text: s}}
			}

			Eventually(grpcServer.envelopes).Should(Receive(&env))
			Expect(env.DeprecatedTags).To(Equal(map[string]*loggregator_v2.Value{
				//client tags
				"origin":     text("a-origin"),
				"deployment": text("a-deployment"),
				"job":        text("a-job"),
				"index":      text("a-index"),
				//metric tags
				"metric_version": text("2.0"),
				"unicorn":        text("another-unicorn"),
			}))
		})

		Context("when the metric is incremented", func() {
			It("emits that value, followed by zero values", func() {
				grpcServer := newgRPCServer()
				defer grpcServer.stop()

				client, err := metricemitter.NewClient(
					grpcServer.addr,
					metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
					metricemitter.WithPulseInterval(50*time.Millisecond),
				)
				Expect(err).ToNot(HaveOccurred())

				metric := client.NewCounter("some-name")
				Eventually(grpcServer.senders).Should(HaveLen(1))

				metric.Increment(5)
				metric.Increment(6)

				var env *loggregator_v2.Envelope
				Eventually(func() uint64 {
					Eventually(grpcServer.envelopes).Should(Receive(&env))
					return env.GetCounter().GetDelta()
				}).Should(Equal(uint64(11)))

				Eventually(grpcServer.envelopes).Should(Receive(&env))
				Expect(env.GetCounter().GetDelta()).To(Equal(uint64(0)))
			})
		})
	})
})

func newgRPCServer() *SpyIngressServer {
	return newgRPCServerWithAddr("127.0.0.1:0")
}

func newgRPCServerWithAddr(addr string) *SpyIngressServer {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	spyIngressServer := &SpyIngressServer{
		addr:      lis.Addr().String(),
		senders:   make(chan loggregator_v2.Ingress_SenderServer, 100),
		envelopes: make(chan *loggregator_v2.Envelope, 100),
		server:    s,
	}

	loggregator_v2.RegisterIngressServer(s, spyIngressServer)
	go func() {
		log.Println(s.Serve(lis))
	}()

	return spyIngressServer
}

type SpyIngressServer struct {
	loggregator_v2.IngressServer

	addr      string
	senders   chan loggregator_v2.Ingress_SenderServer
	server    *grpc.Server
	envelopes chan *loggregator_v2.Envelope
}

func (s *SpyIngressServer) Sender(sender loggregator_v2.Ingress_SenderServer) error {
	s.senders <- sender

	for {
		e, err := sender.Recv()
		if err != nil {
			return err
		}
		s.envelopes <- e
	}
}

func (s *SpyIngressServer) BatchSender(sender loggregator_v2.Ingress_BatchSenderServer) error {
	return nil
}

func (s *SpyIngressServer) Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	return nil, nil
}

func (s *SpyIngressServer) stop() {
	s.server.Stop()
}
