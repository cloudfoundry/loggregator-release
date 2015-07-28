package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"integration_tests/metron/matchers"
	"math/rand"
)

type testServer struct {
	conn        net.PacketConn
	messageChan chan *events.Envelope
	stopChan    chan struct{}
}

func newTestServer(messageChan chan *events.Envelope) *testServer {
	return &testServer{
		conn:        eventuallyListensForUDP("localhost:3457"),
		messageChan: messageChan,
		stopChan:    make(chan struct{}),
	}
}

func (server *testServer) start() {
	for {
		readBuffer := make([]byte, 65535)
		readCount, _, _ := server.conn.ReadFrom(readBuffer)
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])

		if len(readData) > 32 {
			var event events.Envelope
			proto.Unmarshal(readData[32:], &event)
			server.messageChan <- &event
		}

		select {
		case <-server.stopChan:
			return
		default:
		}
	}
}

func (server *testServer) stop() {
	close(server.stopChan)
	server.conn.Close()
}

var _ = Describe("Legacy message forwarding", func() {
	var (
		connection  net.Conn
		messageChan chan *events.Envelope
		testServer  *testServer
	)

	BeforeEach(func() {
		messageChan = make(chan *events.Envelope, 1000)
		testServer = newTestServer(messageChan)

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/0",
			Value: []byte("localhost"),
		}
		adapter := etcdRunner.Adapter()
		adapter.Create(node)

		connection, _ = net.Dial("udp4", "localhost:51160")
		go testServer.start()
	})

	AfterEach(func() {
		testServer.stop()
	})

	It("converts to events message format and forwards to doppler", func(done Done) {
		defer close(done)

		currentTime := time.Now()
		marshalledLegacyMessage := legacyLogMessage(123, "BLAH", currentTime)
		marshalledEventsEnvelope := addDefaultTags(eventsLogMessage(123, "BLAH", currentTime))

		connection.Write(marshalledLegacyMessage)

		stopWrite := make(chan struct{})
		defer close(stopWrite)
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				connection.Write(marshalledLegacyMessage)

				select {
				case <-stopWrite:
					return
				case <-ticker.C:
				}
			}
		}()

		Eventually(messageChan).Should(Receive(Equal(marshalledEventsEnvelope)))
	}, 3)

	It("handles both legacy and dropsonde simultaneously", func() {
		currentTime := time.Now()
		marshalledLegacyMessage := legacyLogMessage(123, "BLAH", currentTime)

		dropsondeConnection, _ := net.Dial("udp4", "localhost:51161")

		go func() {
			for i := 0; i < 1000; i++ {
				connection.Write(marshalledLegacyMessage)
			}
		}()

		rand.Seed(time.Now().UnixNano())

		go func() {
			for i := 0; i < 1000; i++ {
				go func() {
					uuid, _ := uuid.NewV4()
					start := basicHTTPStartMessage(uuid.String())
					dropsondeConnection.Write(start)

					time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)

					stop := basicHTTPStopMessage(uuid.String())
					dropsondeConnection.Write(stop)
				}()
			}
		}()

		unmatchedHttpStop := &events.Envelope{
			EventType: events.Envelope_ValueMetric.Enum(),
			ValueMetric: &events.ValueMetric{
				Name: proto.String("MessageAggregator.httpUnmatchedStartReceived"),
			},
		}

		Consistently(messageChan, 4).ShouldNot(Receive(matchers.MatchSpecifiedContents(unmatchedHttpStop)))
	})
})
