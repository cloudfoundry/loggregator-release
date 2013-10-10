package hasher

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestThatItPanicsWhenNotSeededWithLoggregatorServers(t *testing.T) {
	assert.Panics(t, func() {
		NewHasher([]string{})
	})
}

func TestThatLoggregatorServersReturnsOneServer(t *testing.T) {
	loggregatorServer := []string{"10.10.0.16:9998"}
	h := NewHasher(loggregatorServer)
	assert.Equal(t, loggregatorServer, h.LoggregatorServers())
}

func TestThatLoggregatorServersReturnsAllServers(t *testing.T) {
	loggregatorServers := []string{"10.10.0.16:9998", "10.10.0.17:9997"}
	h := NewHasher(loggregatorServers)
	assert.Equal(t, loggregatorServers, h.LoggregatorServers())
}

func TestThatGetLoggregatorServerForAppIdWorksWithOneServer(t *testing.T) {
	loggregatorServer := []string{"10.10.0.16:9998"}
	h := NewHasher(loggregatorServer)
	ls := h.GetLoggregatorServerForAppId("app1")
	assert.Equal(t, loggregatorServer[0], ls)
}

func TestThatGetLoggregatorServerForAppIdWorksWithTwoServers(t *testing.T) {
	loggregatorServer := []string{"server1", "server2"}

	h := NewHasher(loggregatorServer)
	ls := h.GetLoggregatorServerForAppId("app1")
	assert.Equal(t, loggregatorServer[1], ls)

	ls = h.GetLoggregatorServerForAppId("app2")
	assert.Equal(t, loggregatorServer[0], ls)
}

func TestThatTwoServersActuallyGetUsed(t *testing.T) {
	loggregatorServer := []string{"server1", "server2"}

	h := NewHasher(loggregatorServer)
	ls := h.GetLoggregatorServerForAppId("0")
	assert.Equal(t, loggregatorServer[0], ls)

	ls = h.GetLoggregatorServerForAppId("1")
	assert.Equal(t, loggregatorServer[1], ls)

	ls = h.GetLoggregatorServerForAppId("2")
	assert.Equal(t, loggregatorServer[0], ls)

	ls = h.GetLoggregatorServerForAppId("3")
	assert.Equal(t, loggregatorServer[1], ls)
}

func TestThatMultipleServersGetUsedUniformly(t *testing.T) {
	loggregatorServers := []string{"server1", "server2", "server3"}
	hitCounters := map[string]int{"server1": 0, "server2": 0, "server3": 0}

	h := NewHasher(loggregatorServers)
	target := 1000000
	for i := 0; i < target; i++ {
		ls := h.GetLoggregatorServerForAppId(GenUUID())
		hitCounters[ls] = hitCounters[ls] + 1
	}

	withinLimits := func(target, actual, e int) bool {
		return math.Abs(float64(actual)-float64(target)) < float64(e)
	}

	assert.True(t, withinLimits(target/3, hitCounters["server1"], 30000))
	assert.True(t, withinLimits(target/3, hitCounters["server2"], 30000))
	assert.True(t, withinLimits(target/3, hitCounters["server3"], 30000))
}

func TestThatItIsDeterministic(t *testing.T) {
	loggregatorServers := []string{"10.10.0.16:9998", "10.10.0.17:9997"}
	h := NewHasher(loggregatorServers)

	for i := 0; i < 1000; i++ {
		ls0 := h.GetLoggregatorServerForAppId("appId")
		assert.Equal(t, loggregatorServers[0], ls0)

		ls1 := h.GetLoggregatorServerForAppId("appId23")
		assert.Equal(t, loggregatorServers[1], ls1)
	}
}

func GenUUID() string {
	uuid := make([]byte, 16)
	n, err := rand.Read(uuid)
	if n != len(uuid) || err != nil {
		panic("No GUID generated")
	}
	uuid[8] = 0x80 // variant bits see page 5
	uuid[4] = 0x40 // version 4 Pseudo Random, see page 7

	return hex.EncodeToString(uuid)
}
