package loggregator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"loggregator"
	"loggregator/iprange"
	"testing"
)

func TestLoggregator(t *testing.T) {

	RegisterFailHandler(Fail)

	config := &loggregator.Config{
		Index:                  0,
		IncomingPort:           3456,
		OutgoingPort:           8083,
		LogFilePath:            "",
		MaxRetainedLogMessages: 100,
		WSMessageBufferSize:    100,
		SharedSecret:           "secret",
		SkipCertVerify:         true,
		BlackListIps:           []iprange.IPRange{},
	}
	l := loggregator.New("127.0.0.1", config, loggertesthelper.Logger())
	l.Start()

	RunSpecs(t, "Loggregator Suite")
	l.Stop()
}
