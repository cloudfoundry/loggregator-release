package main_test

import (
	"trafficcontroller"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	Describe("MakeHashers", func() {
		Context("with an empty AZ", func() {
			It("ignores the empty AZ and returns a single hasher", func() {
				config := &main.Config{
					Loggregators: map[string][]string{
						"z1": []string{"10.244.0.14"},
						"z2": []string{},
					},
				}

				hashers := main.MakeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
				Expect(hashers).To(HaveLen(1))
			})

		})

		Context("with two AZs", func() {
			It("returns two hashers", func() {
				config := &main.Config{
					Loggregators: map[string][]string{
						"z1": []string{"10.244.0.14"},
						"z2": []string{"10.244.0.14"},
					},
				}

				hashers := main.MakeHashers(config.Loggregators, 3456, loggertesthelper.Logger())
				Expect(hashers).To(HaveLen(2))
			})
		})
	})

	Describe("ParseConfig", func() {
		var (
			logLevel    = false
			logFilePath = "./test_assets/stdout.log"
		)

		Context("with empty Loggregator ports", func() {
			It("uses defaults", func() {
				configFile := "./test_assets/minimal_loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(8765)))
				Expect(config.LoggregatorIncomingPort).To(Equal(uint32(8765)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(string(config.LoggregatorOutgoingPort)).To(Equal(string(uint32(4567))))
			})
		})

		Context("with specified Loggregator ports", func() {
			It("uses specified ports", func() {
				configFile := "./test_assets/loggregator_trafficcontroller.json"

				var config *main.Config

				config, _, _ = main.ParseConfig(&logLevel, &configFile, &logFilePath)

				Expect(config.IncomingPort).To(Equal(uint32(8765)))
				Expect(config.LoggregatorIncomingPort).To(Equal(uint32(8766)))
				Expect(config.OutgoingPort).To(Equal(uint32(4567)))
				Expect(config.LoggregatorOutgoingPort).To(Equal(uint32(4568)))
			})
		})
	})
})
