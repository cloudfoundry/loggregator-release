package component_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"metron/app"
	"plumbing"
	v2 "plumbing/v2"
	"testservers"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Metron", func() {
	Context("when a consumer is accepting gRPC connections", func() {
		var (
			metronCleanup  func()
			metronConfig   app.Config
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

			go func() {
				for range time.Tick(time.Millisecond) {
					sender.Send(emitEnvelope)
				}
			}()

			var rx v2.DopplerIngress_BatchSenderServer
			Expect(consumerServer.V2.BatchSenderInput.Arg0).Should(Receive(&rx))

			var envBatch *v2.EnvelopeBatch
			var idx int
			f := func() *v2.Envelope {
				batch, err := rx.Recv()
				Expect(err).ToNot(HaveOccurred())

				defer func() { envBatch = batch }()

				for i, envelope := range batch.Batch {
					if envelope.GetLog() != nil {
						idx = i
						return envelope
					}
				}

				return nil
			}
			Eventually(f).ShouldNot(BeNil())

			Expect(len(envBatch.Batch)).ToNot(BeZero())

			Expect(*envBatch.Batch[idx]).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal(&v2.Envelope_Log{
					Log: &v2.Log{Payload: []byte("some-message")},
				}),
				"Tags": Equal(map[string]*v2.Value{
					"auto-tag-1": {
						Data: &v2.Value_Text{"auto-tag-value-1"},
					},
					"auto-tag-2": {
						Data: &v2.Value_Text{"auto-tag-value-2"},
					},
				}),
			}))
		})
	})

	Context("when the consumer is only accepting UDP messages", func() {
		var (
			metronCleanup   func()
			consumerCleanup func()
			metronConfig    app.Config
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

func metronClient(conf app.Config) v2.IngressClient {
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
	return v2.NewIngressClient(conn)
}
