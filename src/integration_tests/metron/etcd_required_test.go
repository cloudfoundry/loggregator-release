package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Etcd dependency", func() {
	Context("when etcd is down", func() {
		BeforeEach(func() {
			etcdRunner.Stop()
		})

		AfterEach(func() {
			etcdRunner.Start()
		})

		It("fails to start", func() {
			configureMetron("tls")
			metronProcess = ifrit.Background(metronRunner)

			Eventually(metronProcess.Wait()).Should(Receive(HaveOccurred()))
			Expect(metronRunner.ExitCode()).NotTo(BeZero())
			Expect(metronRunner.Buffer()).To(gbytes.Say("Error while initializing client pool"))
		})
	})
})
