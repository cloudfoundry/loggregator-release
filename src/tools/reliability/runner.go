package reliability

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

// Reporter is used to report the test results.
type Reporter interface {
	// Report takes the TestResults and submits them.
	Report(t *TestResult) error
}

// Authenticator is used to fetch a token to run the tests with.
type Authenticator interface {
	// Token returns a token to be used for a test.
	Token() (string, error)
}

// LogReliabilityTestRunner runs tests. Each test can be run in parallel to
// each other, and the test result will be submitted to the given Reporter.
// Tokens are required for the tests, which are fetched by the Authenticator.
type LogReliabilityTestRunner struct {
	loggregatorAddr      string
	subscriptionIDPrefix string
	authenticator        Authenticator
	reporter             Reporter
	skipVerify           bool
}

// NewLogReliabilityTestRunner builds a new LogReliabilityTestRunner.
func NewLogReliabilityTestRunner(
	loggregatorAddr string,
	subscriptionIDPrefix string,
	skipVerify bool,
	a Authenticator,
	r Reporter,
) *LogReliabilityTestRunner {
	return &LogReliabilityTestRunner{
		loggregatorAddr:      loggregatorAddr,
		subscriptionIDPrefix: subscriptionIDPrefix,
		skipVerify:           skipVerify,
		authenticator:        a,
		reporter:             r,
	}
}

// Run starts a new test. The test configuration is described by the Test
// type. Each firehose connection has a shardID built by the test ID.
func (r *LogReliabilityTestRunner) Run(t *Test) {
	subscriptionID := fmt.Sprint(r.subscriptionIDPrefix, t.ID)

	authToken, err := r.authenticator.Token()
	if err != nil {
		log.Printf("failed to authenticate with UAA: %s", err)
		return
	}

	cmr := consumer.New(r.loggregatorAddr, &tls.Config{InsecureSkipVerify: r.skipVerify}, nil)
	defer func() {
		if err := cmr.Close(); err != nil {
			log.Printf("failed to close connection to firehose: %v", err)
		}
	}()
	msgChan, errChan := cmr.FirehoseWithoutReconnect(subscriptionID, authToken)

	if !prime(msgChan, errChan, subscriptionID) {
		return
	}

	testLog := []byte(fmt.Sprintf("%s - TEST", subscriptionID))
	go writeLogs(testLog, t.WriteCycles, time.Duration(t.Delay))

	receivedLogCount, err := receiveLogs(
		msgChan,
		errChan,
		testLog,
		t.Cycles,
		time.Duration(t.Timeout),
		subscriptionID,
	)
	if err != nil {
		return
	}

	r.reporter.Report(
		NewTestResult(t, receivedLogCount, time.Now()),
	)
}

func writeLogs(
	logMsg []byte,
	cycles uint64,
	delay time.Duration,
) {
	for i := uint64(0); i < cycles; i++ {
		log.Printf("%s", logMsg)
		time.Sleep(delay)
	}
}

func receiveLogs(
	msgChan <-chan *events.Envelope,
	errChan <-chan error,
	logMsg []byte,
	logCycles uint64,
	timeout time.Duration,
	subscriptionID string,
) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var receivedLogCount uint64
	for {
		select {
		case <-ctx.Done():
			log.Printf("test timedout - %s", subscriptionID)

			return receivedLogCount, nil
		case err := <-errChan:
			if err != nil {
				log.Println(err)
			}

			return 0, err
		case msg := <-msgChan:
			if msg.GetEventType() == events.Envelope_LogMessage {
				if bytes.Contains(msg.GetLogMessage().GetMessage(), logMsg) {
					receivedLogCount++
				}
			}

			if receivedLogCount == logCycles {
				return receivedLogCount, nil
			}
		}
	}
}

func prime(
	msgChan <-chan *events.Envelope,
	errChan <-chan error,
	subscriptionID string,
) bool {
	primerMsg := []byte(fmt.Sprintf("%s - PRIMER", subscriptionID))

	primerTimeout, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	go func() {
		for {
			select {
			case <-primerTimeout.Done():
				return
			default:
				log.Printf("%s", primerMsg)
				time.Sleep(time.Second)
			}
		}
	}()

	for {
		select {
		case <-primerTimeout.Done():
			log.Printf("test timedout while priming - %s", primerMsg)
			return false
		case err := <-errChan:
			if err != nil {
				log.Println(err)
			}

			return false
		case msg := <-msgChan:
			if msg.GetEventType() == events.Envelope_LogMessage {
				if bytes.Contains(msg.GetLogMessage().GetMessage(), primerMsg) {
					return true
				}
			}
		}
	}
}
