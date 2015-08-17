package lats_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"lats/helpers"
	"time"
)

var (
	startTimeStamp = int64(1439589912)
	stopTimeStamp  = int64(1439589916)
	requestId      = &events.UUID{Low: proto.Uint64(1001), High: proto.Uint64(1005)}
	uri            = "http://test.lats"
	remoteAddress  = "127.0.0.1"
	userAgent      = "WebKit"
	statusCode     = int32(200)
	contentLength  = int64(2001)

	httpStart = &events.HttpStart{
		Timestamp:     proto.Int64(startTimeStamp),
		RequestId:     requestId,
		PeerType:      events.PeerType_Client.Enum(),
		Method:        events.Method_GET.Enum(),
		Uri:           proto.String(uri),
		RemoteAddress: proto.String(remoteAddress),
		UserAgent:     proto.String(userAgent),
	}

	httpStop = &events.HttpStop{
		Timestamp:     proto.Int64(stopTimeStamp),
		RequestId:     requestId,
		PeerType:      events.PeerType_Client.Enum(),
		Uri:           proto.String(uri),
		StatusCode:    proto.Int32(statusCode),
		ContentLength: proto.Int64(contentLength),
	}

	httpStartStop = &events.HttpStartStop{
		StartTimestamp: proto.Int64(startTimeStamp),
		StopTimestamp:  proto.Int64(stopTimeStamp),
		RequestId:      requestId,
		PeerType:       events.PeerType_Client.Enum(),
		Method:         events.Method_GET.Enum(),
		Uri:            proto.String(uri),
		RemoteAddress:  proto.String(remoteAddress),
		UserAgent:      proto.String(userAgent),
		StatusCode:     proto.Int32(statusCode),
		ContentLength:  proto.Int64(contentLength),
	}
)

var _ = Describe("Sending Http events through loggregator", func() {

	var (
		msgChan   chan *events.Envelope
		errorChan chan error
	)
	BeforeEach(func() {
		msgChan, errorChan = helpers.ConnectToFirehose()
	})

	AfterEach(func() {
		Expect(errorChan).To(BeEmpty())
	})

	Context("When a Http start/stop event emited into metron", func() {
		It("should expect HttpStartStop metric out of firehose", func() {
			startEnvelope := createHttpStartEvent()
			helpers.EmitToMetron(startEnvelope)

			stopEnvelope := createHttpStopEvent()
			helpers.EmitToMetron(stopEnvelope)

			receivedEnvelop := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelop).NotTo(BeNil())

			receivedHttpStartStopEvent := receivedEnvelop.GetHttpStartStop()

			// not too sure if this deep equal would mach both objects
			Expect(receivedHttpStartStopEvent).To(Equal(httpStartStop))

			Expect(receivedHttpStartStopEvent.GetRequestId()).To(Equal(requestId))
			Expect(receivedHttpStartStopEvent.GetPeerType().String()).To(Equal(events.PeerType_Client.Enum().String()))
			Expect(receivedHttpStartStopEvent.GetMethod().String()).To(Equal(events.Method_GET.Enum().String()))
			Expect(receivedHttpStartStopEvent.GetStartTimestamp()).To(Equal(startTimeStamp))
			Expect(receivedHttpStartStopEvent.GetStopTimestamp()).To(Equal(stopTimeStamp))
			Expect(receivedHttpStartStopEvent.GetUri()).To(Equal(uri))
			Expect(receivedHttpStartStopEvent.GetRemoteAddress()).To(Equal(remoteAddress))
			Expect(receivedHttpStartStopEvent.GetUserAgent()).To(Equal(userAgent))
			Expect(receivedHttpStartStopEvent.GetStatusCode()).To(Equal(statusCode))
			Expect(receivedHttpStartStopEvent.GetContentLength()).To(Equal(contentLength))
		})
	})
})

func createHttpStartEvent() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_HttpStart.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		HttpStart: httpStart,
	}
}

func createHttpStopEvent() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_HttpStop.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		HttpStop:  httpStop,
	}
}
