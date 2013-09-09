package sinks

import (
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func newSimpleRetryStrategy() retryStrategy {
	simple := func(counter int) time.Duration {
		return 1 * time.Millisecond
	}
	return simple
}

func TestConnectWithRetry(t *testing.T) {
	w := &writer{
		appId:         "appId",
		network:       "tcp",
		raddr:         "localhost:3456",
		retryStrategy: newSimpleRetryStrategy(),
		logger:        testhelpers.Logger(),
	}

	err := w.connectWithRetry(0)
	assert.Error(t, err, "Exceeded maximum wait time for establishing a connection to write to.")

	err = w.connectWithRetry(15)
	assert.Error(t, err, "Exceeded maximum wait time for establishing a connection to write to.")

	err = w.connectWithRetry(-20)
	assert.Error(t, err, "Exceeded maximum wait time for establishing a connection to write to.")
}

func TestExponentialRetryStrategy(t *testing.T) {
	strategy := newExponentialRetryStrategy()
	assert.Equal(t, strategy(0).String(), "1ms")
	assert.Equal(t, strategy(1).String(), "2ms")
	assert.Equal(t, strategy(5).String(), "32ms")
	assert.Equal(t, strategy(10).String(), "1.024s")
}
