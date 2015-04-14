package tagger_test

import (
	"metron/tagger"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/localip"
)

var _ = Describe("Tagger", func() {
	It("tags events with the given deployment name, job, index and IP address", func() {
		t := tagger.New("test-deployment", "test-job", 2)

		inputChan := make(chan *events.Envelope)
		outputChan := make(chan *events.Envelope)
		go t.Run(inputChan, outputChan)
		envelope := basicHttpStartStopMessage()
		inputChan <- envelope
		expectedEnvelope := basicTaggedHttpStartStopMessage(*envelope)
		Eventually(outputChan).Should(Receive(Equal(expectedEnvelope)))
	})

	It("overwrites tags already present on the envelope", func() {
		t := tagger.New("test-deployment", "test-job", 2)
		inputChan := make(chan *events.Envelope)
		outputChan := make(chan *events.Envelope)
		go t.Run(inputChan, outputChan)
		envelope := basicHttpStartStopMessage()
		envelope.Tags = []*events.Tag{
			&events.Tag{Key: proto.String("foo"), Value: proto.String("bar")},
			&events.Tag{Key: proto.String("baz"), Value: proto.String("bang")},
		}

		inputChan <- envelope
		expectedEnvelope := basicTaggedHttpStartStopMessage(*envelope)
		Eventually(outputChan).Should(Receive(Equal(expectedEnvelope)))
	})

})

func basicHttpStartStopMessage() *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_HttpStartStop.Enum(),
		HttpStartStop: &events.HttpStartStop{
			StartTimestamp: proto.Int64(1234),
			StopTimestamp:  proto.Int64(5555),
			RequestId: &events.UUID{
				Low:  proto.Uint64(11),
				High: proto.Uint64(12),
			},
			PeerType:      events.PeerType_Server.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("http://test.example.com"),
			RemoteAddress: proto.String("http://test.example.com"),
			UserAgent:     proto.String("test"),
			StatusCode:    proto.Int32(1234),
			ContentLength: proto.Int64(5678),
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(11),
				High: proto.Uint64(12),
			},
		},
	}
}

func basicTaggedHttpStartStopMessage(envelope events.Envelope) *events.Envelope {
	ip, _ := localip.LocalIP()
	envelope.Tags = []*events.Tag{
		&events.Tag{
			Key:   proto.String("deployment"),
			Value: proto.String("test-deployment"),
		},
		&events.Tag{
			Key:   proto.String("job"),
			Value: proto.String("test-job"),
		},
		&events.Tag{
			Key:   proto.String("index"),
			Value: proto.String("2"),
		},
		&events.Tag{
			Key:   proto.String("ip"),
			Value: proto.String(ip),
		}}
	return &envelope
}
