package lats_test

import (
	"crypto/tls"
	"lats/helpers"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry/noaa/consumer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	It("gets through recent logs", func() {

		Expect(cf.Cf("api", "api."+config.AppDomain, "--skip-ssl-validation").Wait(cfSetupTimeOut)).To(Exit(0))
		Expect(cf.Cf("auth", config.AdminUser, config.AdminPassword).Wait(cfSetupTimeOut)).To(Exit(0))
		Expect(cf.Cf("create-org", "lats").Wait(cfSetupTimeOut)).To(Exit(0))
		Expect(cf.Cf("create-space", "-o", "lats", "lats").Wait(cfSetupTimeOut)).To(Exit(0))
		Expect(cf.Cf("target", "-o", "lats", "-s", "lats").Wait(cfSetupTimeOut)).To(Exit(0))
		Expect(cf.Cf("push", "lumberjack", "--no-start",
			"-b", "go_buildpack",
			"-m", defaultMemoryLimit,
			"-p", "../tools/lumberjack",
			"-d", config.AppDomain).Wait(cfPushTimeOut)).To(Exit(0))

		sess := cf.Cf("app", "lumberjack", "--guid")
		Expect(sess.Wait(cfSetupTimeOut)).To(Exit(0))
		appGuid := strings.TrimSpace(string(sess.Out.Contents()))

		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

		token := helpers.GetAuthToken()
		envelopes, err := consumer.RecentLogs(appGuid, token)
		Expect(err).NotTo(HaveOccurred())
		Expect(envelopes).ToNot(BeEmpty())
	})
})
