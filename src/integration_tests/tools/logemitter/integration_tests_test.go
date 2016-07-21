package integration_tests_test

import (
	"integration_tests/tools/helpers"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("IntegrationTests", func() {
	Describe("Web requests", func() {
		It("returns 200 for healthchecks", func() {
			logfinSession, port := helpers.StartLogfin()
			logemitterSession, _ := helpers.StartLogemitter(port)

			Eventually(func() bool { return checkEndpoint(port, "") }).Should(Equal(true))
			resp, err := http.Get("http://localhost:" + port)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			logfinSession.Kill().Wait()
			logemitterSession.Kill().Wait()
		})
	})

	Describe("Log emitting", func() {
		It("emits logs", func() {
			logfinSession, port := helpers.StartLogfin()
			logemitterSession, _ := helpers.StartLogemitter(port)

			Eventually(logemitterSession.Buffer).Should(gbytes.Say("logemitter guid: .* msg: [/d]*"))

			logfinSession.Kill().Wait()
			logemitterSession.Kill().Wait()
		})
	})

	Describe("Signalling completion", func() {
		It("notifies logfin when it has finished emitting logs", func() {
			logfinSession, port := helpers.StartLogfin()
			logemitterSession, _ := helpers.StartLogemitter(port)

			Eventually(func() bool { return checkEndpoint(port, "") }).Should(Equal(true))

			By("getting a 200 status from logfin after sending completion status")
			Eventually(func() bool { return checkEndpoint(port, "status") }).Should(Equal(true))

			logfinSession.Kill().Wait()
			logemitterSession.Kill().Wait()
		})
	})
})

func checkEndpoint(port, endpoint string) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		return resp.StatusCode == http.StatusOK
	}

	return false
}
