package client_test

import (
	"time"
	"tools/reliability/worker/internal/client"
	"tools/reliability/worker/internal/reporter"

	sharedapi "tools/reliability/api"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Run()", func() {
	It("passes through the test start time to the test result", func() {
		spyRep := &spyReporter{}
		spyConsumer := NewSpyConsumer()
		runner := client.NewLogReliabilityTestRunner(
			"fh",
			"subscriptionID",
			&spyAuthenticator{},
			spyRep,
			spyConsumer,
		)
		startTime := time.Now()

		primerLog := events.Envelope{
			Origin:    proto.String("origin"),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Message:     []byte("subscriptionID0 - PRIMER"),
				MessageType: events.LogMessage_OUT.Enum(),
				Timestamp:   proto.Int64(startTime.UnixNano()),
			},
		}

		spyConsumer.msgChan <- &primerLog

		runner.Run(&sharedapi.Test{
			Cycles:    12413,
			StartTime: startTime,
		})

		Expect(spyRep.results.TestStartTime).To(Equal(startTime))
	})
})

type spyReporter struct {
	results reporter.TestResult
}

func (s *spyReporter) Report(t *reporter.TestResult) error {
	s.results = *t
	return nil
}

type spyAuthenticator struct{}

func (s spyAuthenticator) Token() (string, error) {
	return "", nil
}

type spyConsumer struct {
	msgChan chan *events.Envelope
	errChan chan error
}

func NewSpyConsumer() *spyConsumer {
	return &spyConsumer{
		msgChan: make(chan *events.Envelope, 1),
		errChan: make(chan error, 1),
	}
}

func (s *spyConsumer) FirehoseWithoutReconnect(string, string) (<-chan *events.Envelope, <-chan error) {
	return s.msgChan, s.errChan
}
