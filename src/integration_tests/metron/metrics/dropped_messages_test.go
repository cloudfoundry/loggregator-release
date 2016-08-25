package metrics_test

import (
	"bytes"
	dopplerconfig "doppler/config"
	"doppler/dopplerservice"
	"fmt"
	"integration_tests/metron/metrics"
	"math/rand"
	"metron/config"
	"net"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DroppedMessages", func() {
	var (
		testDoppler  *metrics.TCPTestDoppler
		metronInput  net.Conn
		stopAnnounce chan (chan bool)
	)

	BeforeEach(func() {
		dopplerPort := rand.Intn(55536) + 10000
		dopplerAddr := fmt.Sprintf("127.0.0.1:%d", dopplerPort)
		testDoppler = metrics.NewTCPTestDoppler(dopplerAddr)
		go testDoppler.Start()

		dopplerConfig := &dopplerconfig.Config{
			Index:           "0",
			JobName:         "job",
			Zone:            "z9",
			IncomingTCPPort: uint32(dopplerPort),
		}

		stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, gosteno.NewLogger("test"))

		metronRunner.Protocols = config.Protocols{"tcp": struct{}{}}
		metronRunner.Start()

		var err error
		metronInput, err = net.Dial("udp4", metronRunner.MetronAddress())
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		metronInput.Close()
		testDoppler.Stop()
		close(stopAnnounce)
		metronRunner.Stop()
	})

	It("emits logs when it drops messages due to a slow doppler", func() {
		By("allowing the channel send in the test doppler to block")
		for i := 0; i < 100000; i++ {
			writeToMetron(metronInput)
		}

		By("reading messages from the test doppler until we find the dropped counts")
		counter, log := findDroppedCounterAndLog(testDoppler.MessageChan, 10*time.Second)
		Expect(counter).ToNot(BeNil())
		Expect(log).ToNot(BeNil())
		Expect(log.Message).To(BeEquivalentTo(fmt.Sprintf("Dropped %d message(s) from MetronAgent to Doppler", counter.GetDelta())))
	})
})

func findDroppedCounterAndLog(messages <-chan *events.Envelope, timeout time.Duration) (*events.CounterEvent, *events.LogMessage) {
	var (
		counter *events.CounterEvent
		log     *events.LogMessage
	)
	until := time.After(timeout)
	for {
		select {
		case ev := <-messages:
			switch ev.GetEventType() {
			case events.Envelope_CounterEvent:
				if ev.CounterEvent.GetName() == "DroppedCounter.droppedMessageCount" {
					counter = ev.CounterEvent
				}
			case events.Envelope_LogMessage:
				if bytes.HasPrefix(ev.LogMessage.Message, []byte("Dropped ")) {
					log = ev.LogMessage
				}
			}
			if counter != nil && log != nil {
				return counter, log
			}
		case <-until:
			return nil, nil
		}
	}
}

func writeToMetron(metronInput net.Conn) error {
	dropsondeMessage := &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("some test log message"),
			MessageType: events.LogMessage_ERR.Enum(),
			Timestamp:   proto.Int64(1),
		},
	}
	message, err := proto.Marshal(dropsondeMessage)
	if err != nil {
		return err
	}
	_, err = metronInput.Write(message)
	return err
}
