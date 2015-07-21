package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	"bytes"
	"fmt"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

var _ = Describe("Dropsonde message forwarding", func() {
	var testDoppler net.PacketConn
	var etcdAdapter storeadapter.StoreAdapter
	var messageChan chan signedMessage
	var stopTheWorld chan struct{}

	BeforeEach(func() {
		testDoppler = eventuallyListensForUDP("localhost:3457")

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/0",
			Value: []byte("localhost"),
		}

		messageChan = make(chan signedMessage, 1000)
		stopTheWorld = make(chan struct{})

		etcdAdapter = etcdRunner.Adapter()
		etcdAdapter.Create(node)
	})

	AfterEach(func() {
		etcdAdapter.Disconnect()
		testDoppler.Close()
	})

	It("forwards hmac signed messages to a healthy doppler server", func(done Done) {

		defer close(done)
		defer close(messageChan)
		defer close(stopTheWorld)

		valueMetricEnvelope := createBasicValueMetricEnvelope()
		expectedSignedEnvelope := sign(valueMetricEnvelope)

		go readFromDoppler(len(expectedSignedEnvelope.signature), testDoppler, messageChan, stopTheWorld)
		go writeToMetron(stopTheWorld)

		Eventually(func() bool {
			msg := <-messageChan
			return bytes.Equal(msg.signature, expectedSignedEnvelope.signature) && bytes.Equal(msg.message, expectedSignedEnvelope.message)
		}).Should(BeTrue())
	}, 2)

	It("forwards multiple messages to metron concurrently", func(done Done) {

		defer close(done)
		defer close(messageChan)
		defer close(stopTheWorld)

		numOfMsgs := 100

		signedEnvelopes := make([]signedMessage, 0, numOfMsgs)
		for i := 0; i < numOfMsgs; i++ {
			envelope := createLogMsgEnvelope(i)
			signedEnvelopes = append(signedEnvelopes, sign(envelope))
		}

		// read from doppler
		go readFromDoppler(len(signedEnvelopes[0].signature), testDoppler, messageChan, stopTheWorld)

		// write to metron
		for _, signedEnvelope := range signedEnvelopes {
			go writeLogMsgToMetron(signedEnvelope)
		}

		// read messageChan
		actualMsgs := make([]signedMessage, 0, numOfMsgs)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			count := 0
			for msg := range messageChan {
				count++
				actualMsgs = append(actualMsgs, msg)
				if count == numOfMsgs {
					wg.Done()
					break
				}
			}
		}()

		wg.Wait()
		Expect(actualMsgs).To(HaveLen(numOfMsgs))

		for _, actual := range actualMsgs {
			Expect(signedEnvelopes).To(ContainElement(actual))
		}
	})
})

func createBasicValueMetricEnvelope() *events.Envelope {
	return addDefaultTags(basicValueMessageEnvelope())
}

func createLogMsgEnvelope(appid int) *events.Envelope {
	return addDefaultTags(eventsLogMessage(appid, fmt.Sprintf("%s %d", "Test", appid), time.Now()))
}

func sign(envelope *events.Envelope) signedMessage {
	mac := hmac.New(sha256.New, []byte("shared_secret"))
	b, _ := proto.Marshal(envelope)
	mac.Write(b)

	s := mac.Sum(nil)
	return signedMessage{signature: s, message: b}
}

func writeLogMsgToMetron(signedMsg signedMessage) {
	metronInput, err := net.Dial("udp", "localhost:51161")
	if err != nil {
		panic(err.Error())
	}

	metronInput.Write(signedMsg.message)
}

func writeToMetron(stopTheWorld chan struct{}) {
	metronInput, _ := net.Dial("udp", "localhost:51161")
	ticker := time.NewTicker(10 * time.Millisecond)

	originalMessage := basicValueMessage()

	for {
		metronInput.Write(originalMessage)

		select {
		case <-stopTheWorld:
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}

func readFromDoppler(signatureLen int, testDoppler net.PacketConn, messageChan chan signedMessage, stopTheWorld chan struct{}) {
	gotSignedMessage := func(readData []byte) bool {
		return len(readData) > signatureLen
	}

	readBuffer := make([]byte, 65535)

	for {
		readCount, _, _ := testDoppler.ReadFrom(readBuffer)
		readData := make([]byte, readCount)
		copy(readData, readBuffer[:readCount])

		if gotSignedMessage(readData) {
			messageChan <- signedMessage{signature: readData[:signatureLen], message: readData[signatureLen:]}
		}

		select {
		case <-stopTheWorld:
			return
		default:
		}
	}
}

type signedMessage struct {
	signature []byte
	message   []byte
}
