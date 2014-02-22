package main

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/stretchr/testify/assert"
	"testing"
)

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

func TestConfigWithEmptyLoggregatorIncomingPort(t *testing.T) {
	config := &Config{
		IncomingPort: 3456,
		SystemDomain: "192.168.1.10.xip.io",
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{"10.244.0.14"},
		},
	}

	assert.NotPanics(t, func() {
		config.validate(loggertesthelper.Logger())
	})
	assert.Equal(t, config.IncomingPort, uint32(3456))
	assert.Equal(t, config.LoggregatorIncomingPort, uint32(3456))
}

func TestConfigWithLoggregatorIncomingPort(t *testing.T) {
	config := &Config{
		IncomingPort:            3456,
		LoggregatorIncomingPort: 13456,
		SystemDomain:            "192.168.1.10.xip.io",
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{"10.244.0.14"},
		},
	}

	assert.NotPanics(t, func() {
		config.validate(loggertesthelper.Logger())
	})
	assert.Equal(t, config.IncomingPort, uint32(3456))
	assert.Equal(t, config.LoggregatorIncomingPort, uint32(13456))
}

func TestConfigWithEmptyLoggregatorOutgoingPort(t *testing.T) {
	config := &Config{
		OutgoingPort: 8080,
		SystemDomain: "192.168.1.10.xip.io",
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{"10.244.0.14"},
		},
	}

	assert.NotPanics(t, func() {
		config.validate(loggertesthelper.Logger())
	})
	assert.Equal(t, config.OutgoingPort, uint32(8080))
	assert.Equal(t, config.LoggregatorOutgoingPort, uint32(8080))
}

func TestConfigWithLoggregatorOutgoingPort(t *testing.T) {
	config := &Config{
		OutgoingPort:            8080,
		LoggregatorOutgoingPort: 18080,
		SystemDomain:            "192.168.1.10.xip.io",
		Loggregators: map[string][]string{
			"z1": []string{"10.244.0.14"},
			"z2": []string{"10.244.0.14"},
		},
	}

	assert.NotPanics(t, func() {
		config.validate(loggertesthelper.Logger())
	})
	assert.Equal(t, config.OutgoingPort, uint32(8080))
	assert.Equal(t, config.LoggregatorOutgoingPort, uint32(18080))
}
