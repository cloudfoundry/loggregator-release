package component_test

import (
	"context"
	"fmt"
	"metron/api"
	"net"
	"plumbing"
	v2 "plumbing/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"testservers"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metron", func() {
	Context("when a consumer is accepting gRPC connections", func() {
		var (
			metronCleanup  func()
			metronConfig   api.Config
			consumerServer *Server
			eventEmitter   dropsonde.EventEmitter
		)

		BeforeEach(func() {
			var err error
			consumerServer, err = NewServer()
			Expect(err).ToNot(HaveOccurred())

			var metronReady func()
			metronCleanup, metronConfig, metronReady = testservers.StartMetron(
				testservers.BuildMetronConfig("localhost", consumerServer.Port(), 0),
			)
			defer metronReady()
		})

		AfterEach(func() {
			consumerServer.Stop()
			metronCleanup()
		})

		It("accepts connections on the v1 API", func() {
			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronConfig.IncomingUDPPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")

			emitEnvelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			f := func() int {
				eventEmitter.Emit(emitEnvelope)
				return len(consumerServer.V1.PusherCalled)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))

			var rx plumbing.DopplerIngestor_PusherServer
			Expect(consumerServer.V1.PusherInput.Arg0).Should(Receive(&rx))

			data, err := rx.Recv()
			Expect(err).ToNot(HaveOccurred())

			envelope := new(events.Envelope)
			Expect(envelope.Unmarshal(data.Payload)).To(Succeed())
		})

		It("accepts connections on the v2 API", func() {
			emitEnvelope := &v2.Envelope{
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("some-message"),
						Type:    v2.Log_OUT,
					},
				},
			}

			client := metronClient(metronConfig)
			sender, err := client.Sender(context.Background())
			Expect(err).ToNot(HaveOccurred())

			Consistently(func() error {
				return sender.Send(emitEnvelope)
			}, 5).Should(Succeed())

			var rx v2.DopplerIngress_SenderServer
			Expect(consumerServer.V2.SenderInput.Arg0).Should(Receive(&rx))

			f := func() *v2.Envelope {
				envelope, err := rx.Recv()
				Expect(err).ToNot(HaveOccurred())
				return envelope
			}
			Eventually(f).Should(Equal(emitEnvelope))
		})

		It("emits metrics to the v2 API", func() {
			emitEnvelope := &v2.Envelope{
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("some-message"),
						Type:    v2.Log_OUT,
					},
				},
			}

			client := metronClient(metronConfig)
			ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
			sender, err := client.Sender(ctx)
			Expect(err).ToNot(HaveOccurred())

			Consistently(func() error {
				return sender.Send(emitEnvelope)
			}, 5).Should(Succeed())

			var rx v2.DopplerIngress_SenderServer
			Expect(consumerServer.V2.SenderInput.Arg0).Should(Receive(&rx))

			f := func() bool {
				envelope, err := rx.Recv()
				Expect(err).ToNot(HaveOccurred())

				return envelope.GetCounter() != nil &&
					envelope.GetCounter().GetDelta() > 0
			}
			Eventually(f).Should(Equal(true))
		})
	})

	Context("when the consumer is only accepting UDP messages", func() {
		var (
			metronCleanup   func()
			consumerCleanup func()
			metronConfig    api.Config
			udpPort         int
			eventEmitter    dropsonde.EventEmitter
			consumerConn    *net.UDPConn
		)

		BeforeEach(func() {
			addr, err := net.ResolveUDPAddr("udp4", "localhost:0")
			Expect(err).ToNot(HaveOccurred())
			consumerConn, err = net.ListenUDP("udp4", addr)
			Expect(err).ToNot(HaveOccurred())
			udpPort = HomeAddrToPort(consumerConn.LocalAddr())
			consumerCleanup = func() {
				consumerConn.Close()
			}

			var metronReady func()
			metronCleanup, metronConfig, metronReady = testservers.StartMetron(
				testservers.BuildMetronConfig("localhost", 0, udpPort),
			)
			defer metronReady()

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronConfig.IncomingUDPPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")
		})

		AfterEach(func() {
			consumerCleanup()
			metronCleanup()
		})

		It("writes to the consumer via UDP", func() {
			envelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			c := make(chan bool, 100)
			var wg sync.WaitGroup
			wg.Add(1)
			defer wg.Wait()
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				buffer := make([]byte, 1024)
				for {
					consumerConn.SetReadDeadline(time.Now().Add(5 * time.Second))
					_, err := consumerConn.Read(buffer)
					Expect(err).ToNot(HaveOccurred())

					select {
					case c <- true:
					default:
						return
					}
				}
			}()

			f := func() int {
				eventEmitter.Emit(envelope)
				return len(c)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))
		})
	})
})

func HomeAddrToPort(addr net.Addr) int {
	port, err := strconv.Atoi(strings.Replace(addr.String(), "127.0.0.1:", "", 1))
	if err != nil {
		panic(err)
	}
	return port
}

func metronClient(conf api.Config) v2.MetronIngressClient {
	addr := fmt.Sprintf("127.0.0.1:%d", conf.GRPC.Port)

	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		panic(err)
	}
	return v2.NewMetronIngressClient(conn)
}
