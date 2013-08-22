package main

import (
	"github.com/stretchr/testify/assert"
	"testhelpers"
	"testing"
)

type MockLoggregatorClient struct {
	received chan *[]byte
}

func (m MockLoggregatorClient) Send(data []byte) {
	m.received <- &data
}

func (m MockLoggregatorClient) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

func (m MockLoggregatorClient) IncLogStreamRawByteCount(uint64) {

}

func (m MockLoggregatorClient) IncLogStreamPbByteCount(uint64) {

}

func TestThatItHashesForTwoServers(t *testing.T) {
	mockLoggregatorClient := new(MockLoggregatorClient)
	loggregatorServers := []string{"1.2.3.4:6789", "2.3.4.5:8434"}
	h := newHasher(loggregatorServers, testhelpers.Logger())
	appid := "appid"
	appid2 := "appid2"
	assert.Equal(t, h.lookupLoggregatorClientForAppId(appid), loggregatorServers[0])
	assert.Equal(t, h.lookupLoggregatorClientForAppId(appid), loggregatorServers[1])
}

// func TestThatItHashesForOneServer(t *testing.T) {
// loggregatorServers := []string{"1.2.3.4:6789"}
// appid := "appid"
// }
