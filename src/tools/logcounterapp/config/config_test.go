package config_test

import (
	"os"
	"time"
	"tools/logcounterapp/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("ParseEnv", func() {
		It("parses environment variables", func() {
			setAllEnvsExcept("")

			cfg, err := config.ParseEnv()
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Username).To(Equal("testUserName"))
			Expect(cfg.Password).To(Equal("testPassword"))
			Expect(cfg.ClientID).To(Equal("testID"))
			Expect(cfg.ClientSecret).To(Equal("clientSecret"))
			Expect(cfg.MessagePrefix).To(Equal("testPrefix"))
			Expect(cfg.DopplerURL).To(Equal("doppler.test.com"))
			Expect(cfg.ApiURL).To(Equal("api.test.com"))
			Expect(cfg.UaaURL).To(Equal("uaa.test.com"))
			Expect(cfg.Port).To(Equal("12345"))
			Expect(cfg.LogfinURL).To(Equal("logfin.test.com"))
			Expect(cfg.Runtime).To(Equal(time.Second))
		})

		It("returns an error for an invalid runtime duration", func() {
			setAllEnvsExcept("RUNTIME")
			os.Setenv("RUNTIME", "foo")

			cfg, err := config.ParseEnv()
			Expect(cfg).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("returns error for missing required properties", func(property string) {
			Expect(setAllEnvsExcept(property)).ToNot(HaveOccurred())
			cfg, err := config.ParseEnv()
			Expect(cfg).To(BeNil())
			Expect(err).To(HaveOccurred())
		},
			Entry("dopplerAddress", "DOPPLER_URL"),
			Entry("apiAddress", "API_URL"),
			Entry("uaaAddress", "UAA_URL"),
			Entry("clientID", "CLIENT_ID"),
			Entry("logcounter port", "PORT"),
			Entry("logfinAddress", "LOGFIN_URL"),
			Entry("runtime", "RUNTIME"),
		)

		It("generates a random SubscriptionID if none is provided", func() {
			Expect(setAllEnvsExcept("SUBSCRIPTION_ID")).ToNot(HaveOccurred())
			cfg, err := config.ParseEnv()
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.SubscriptionID).ToNot(BeEmpty())
		})
	})
})

func setAllEnvsExcept(remove string) error {
	os.Unsetenv(remove)
	allEnvs := map[string]string{
		"API_URL":         "api.test.com",
		"DOPPLER_URL":     "doppler.test.com",
		"UAA_URL":         "uaa.test.com",
		"CLIENT_ID":       "testID",
		"CLIENT_SECRET":   "clientSecret",
		"CF_USERNAME":     "testUserName",
		"CF_PASSWORD":     "testPassword",
		"MESSAGE_PREFIX":  "testPrefix",
		"SUBSCRIPTION_ID": "testSubID",
		"PORT":            "12345",
		"LOGFIN_URL":      "logfin.test.com",
		"RUNTIME":         "1s",
	}
	for k, v := range allEnvs {
		if k == remove {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}
