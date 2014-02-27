package main

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/stretchr/testify/assert"
	"testing"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/gosteno"
)

type FakeYagnatsClient struct {

}

func (c *FakeYagnatsClient) Ping() bool { return true}
func (c *FakeYagnatsClient) Connect(connectionProvider yagnats.ConnectionProvider) error {return nil}
func (c *FakeYagnatsClient) Disconnect() {}
func (c *FakeYagnatsClient) Publish(subject string, payload []byte) error {return nil}
func (c *FakeYagnatsClient) PublishWithReplyTo(subject, reply string, payload []byte) error {return nil}
func (c *FakeYagnatsClient) Subscribe(subject string, callback yagnats.Callback) (int, error) { return 0, nil }
func (c *FakeYagnatsClient) SubscribeWithQueue(subject, queue string, callback yagnats.Callback) (int, error) {return 0, nil}
func (c *FakeYagnatsClient) Unsubscribe(subscription int) error {return nil}
func (c *FakeYagnatsClient) UnsubscribeAll(subject string) {}

func TestOutgoingProxyConfigWithEmptyAZ(t *testing.T) {
	config := &Config{
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{},
		},
	}

	assert.NotPanics(t, func() {
		makeOutgoingProxy("0.0.0.0", config, loggertesthelper.Logger())
	})

	hashers := makeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
	assert.Equal(t, len(hashers), 1)
}

func TestOutgoingProxyConfigWithTwoAZs(t *testing.T) {
	config := &Config{
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{"10.244.0.14"},
		},
	}

	assert.NotPanics(t, func() {
		makeOutgoingProxy("0.0.0.0", config, loggertesthelper.Logger())
	})

	hashers := makeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
	assert.Equal(t, len(hashers), 2)
}

func TestConfigWithEmptyLoggregatorPorts(t *testing.T) {
	cfcomponent.DefaultYagnatsClientProvider = func (logger *gosteno.Logger) yagnats.NATSClient {
		return &FakeYagnatsClient{}
	}

	logLevel := false
	configFile := "./test_assets/minimal_loggregator_trafficcontroller.json"
	logFilePath := "./test_assets/stdout.log"

	var config *Config

	assert.NotPanics(t, func() {
			config, _, _ = parseConfig(&logLevel, &configFile, &logFilePath)
		})
	assert.Equal(t, config.IncomingPort, uint32(8765))
	assert.Equal(t, config.LoggregatorIncomingPort, uint32(8765))
	assert.Equal(t, config.OutgoingPort, uint32(4567))
	assert.Equal(t, string(config.LoggregatorOutgoingPort), string(uint32(4567)))
}

func TestConfigWithSpecifiedLoggregatorPorts(t *testing.T) {
	cfcomponent.DefaultYagnatsClientProvider = func (logger *gosteno.Logger) yagnats.NATSClient {
		return &FakeYagnatsClient{}
	}

	logLevel := false
	configFile := "./test_assets/loggregator_trafficcontroller.json"
	logFilePath := "./test_assets/stdout.log"

	var config *Config

	assert.NotPanics(t, func() {
			config, _, _ = parseConfig(&logLevel, &configFile, &logFilePath)
		})
	assert.Equal(t, config.IncomingPort, uint32(8765))
	assert.Equal(t, config.LoggregatorIncomingPort, uint32(8766))
	assert.Equal(t, config.OutgoingPort, uint32(4567))
	assert.Equal(t, config.LoggregatorOutgoingPort, uint32(4568))
}
