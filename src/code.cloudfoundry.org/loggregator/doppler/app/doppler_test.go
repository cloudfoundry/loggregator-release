package app_test

import (
	"code.cloudfoundry.org/loggregator/doppler/app"
	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Doppler", func() {
	Describe("Addrs()", func() {
		It("returns a struct with all the addrs", func() {
			grpc := app.GRPC{
				CAFile:   testservers.Cert("loggregator-ca.crt"),
				CertFile: testservers.Cert("doppler.crt"),
				KeyFile:  testservers.Cert("doppler.key"),
			}

			config := &app.Config{
				GRPC: grpc,
				MetronConfig: app.MetronConfig{
					UDPAddress:  "127.0.0.1:3457",
					GRPCAddress: "127.0.0.1:3458",
				},
				HealthAddr:                      ":0",
				MaxRetainedLogMessages:          100,
				MessageDrainBufferSize:          10000,
				MetricBatchIntervalMilliseconds: 1000,
				SinkInactivityTimeoutSeconds:    3600,
				ContainerMetricTTLSeconds:       120,
				DisableAnnounce:                 true,
				DisableSyslogDrains:             true,
			}

			doppler := app.NewLegacyDoppler(config)
			doppler.Start()

			addrs := doppler.Addrs()

			Expect(addrs.Health).ToNot(Equal(""))
			Expect(addrs.Health).ToNot(Equal("0.0.0.0:0"))
			Expect(addrs.GRPC).ToNot(Equal(""))
			Expect(addrs.GRPC).ToNot(Equal("0.0.0.0:0"))
		})
	})
})
