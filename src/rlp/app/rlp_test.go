package app_test

import (
	"context"
	"log"
	"net"
	"plumbing"
	v2 "plumbing/v2"
	app "rlp/app"
	"time"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Start", func() {
	It("receive messages via egress client", func() {
		// create fake doppler
		doppler := newMockDopplerServer()

		lis, err := net.Listen("tcp", "localhost:0")
		defer lis.Close()
		Expect(err).ToNot(HaveOccurred())

		grpcServer := grpc.NewServer()
		plumbing.RegisterDopplerServer(grpcServer, doppler)
		go grpcServer.Serve(lis)

		egressLis, err := net.Listen("tcp", "localhost:0")
		egressLis.Close()
		Expect(err).ToNot(HaveOccurred())

		egressPort := egressLis.Addr().(*net.TCPAddr).Port
		// point our app at the fake doppler
		rlp := app.NewRLP(
			app.WithIngressAddrs([]string{lis.Addr().String()}),
			app.WithEgressPort(egressPort),
		)
		go rlp.Start()

		// create public client consuming log API
		conn, err := grpc.Dial(
			egressLis.Addr().String(),
			grpc.WithInsecure(),
		)
		Expect(err).ToNot(HaveOccurred())

		defer conn.Close()
		egressRequest := &v2.EgressRequest{}
		egressClient := v2.NewEgressClient(conn)

		var egressStream v2.Egress_ReceiverClient
		Eventually(func() error {
			egressStream, err = egressClient.Receiver(context.Background(), egressRequest)
			return err
		}, 5).ShouldNot(HaveOccurred())

		var subscriber plumbing.Doppler_SubscribeServer
		Eventually(doppler.SubscribeInput.Stream, 5).Should(Receive(&subscriber))

		go func() {
			payload := buildLogMessage()
			response := &plumbing.Response{
				Payload: payload,
			}

			for {
				err := subscriber.Send(response)
				if err != nil {
					log.Printf("subscriber#Send failed: %s\n", err)
					return
				}
			}
		}()

		envelope, err := egressStream.Recv()
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.GetTags()["origin"].GetText()).To(Equal("some-origin"))
	})
})

func buildLogMessage() []byte {
	e := &events.Envelope{
		Origin:    proto.String("some-origin"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("foo"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String("test-app"),
		},
	}
	b, _ := proto.Marshal(e)
	return b
}
