package lats_test

import (
	"fmt"
	"lats/helpers"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/instrumented_handler"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Context("When the instrumented handler receives a request", func() {
		It("should emit an HttpStartStop through the firehose", func() {
			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("localhost:%d", config.DropsondePort))
			Expect(err).ToNot(HaveOccurred())
			emitter := emitter.NewEventEmitter(udpEmitter, helpers.ORIGIN_NAME)
			done := make(chan struct{})
			handler := instrumented_handler.InstrumentedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond)
				w.WriteHeader(http.StatusTeapot)
				close(done)
			}), emitter)
			r, err := http.NewRequest("HEAD", "/", nil)
			Expect(err).ToNot(HaveOccurred())
			r.Header.Add("User-Agent", "Spider-Man")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			Eventually(done).Should(BeClosed())

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())
			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_HttpStartStop))

			event := receivedEnvelope.GetHttpStartStop()
			Expect(event.GetPeerType().String()).To(Equal(events.PeerType_Server.Enum().String()))
			Expect(event.GetMethod().String()).To(Equal(events.Method_HEAD.Enum().String()))
			Expect(event.GetStopTimestamp() - event.GetStartTimestamp()).To(BeNumerically("~", 100*time.Millisecond, time.Millisecond/2))
			Expect(event.GetUserAgent()).To(Equal("Spider-Man"))
			Expect(event.GetStatusCode()).To(BeEquivalentTo(http.StatusTeapot))
		})
	})
})
