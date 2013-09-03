package loggregatorrouter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestThatItPanicsWhenNotSeededWithLoggregatorServers(t *testing.T) {
	assert.Panics(t, func() {
		NewHasher([]string{})
	})
}

func TestThatItLooksUpLoggregatorServerForAppId(t *testing.T) {
	loggregatorServers := []string{"10.10.0.16:9998", "10.10.0.17:9997"}
	h := NewHasher(loggregatorServers)

	ls0, err := h.getLoggregatorServerForAppId("appId")
	assert.NoError(t, err)
	assert.Equal(t, loggregatorServers[0], ls0)

	ls1, err := h.getLoggregatorServerForAppId("xsdfappId")
	assert.NoError(t, err)
	assert.Equal(t, loggregatorServers[1], ls1)

	// Test that we're still getting the same values
	ls0, err = h.getLoggregatorServerForAppId("appId")
	assert.NoError(t, err)
	assert.Equal(t, loggregatorServers[0], ls0)

	ls1, err = h.getLoggregatorServerForAppId("xsdfappId")
	assert.NoError(t, err)
	assert.Equal(t, loggregatorServers[1], ls1)
}

func TestThatItReturnsSeededLoggregatorServers(t *testing.T) {
	loggregatorServers := []string{"10.10.0.16:9998", "10.10.0.17:9997"}
	h := NewHasher(loggregatorServers)
	assert.Equal(t, loggregatorServers, h.loggregatorServers())
}
