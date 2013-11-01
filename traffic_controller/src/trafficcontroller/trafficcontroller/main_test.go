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
