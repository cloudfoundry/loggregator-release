package integration_tests_test

import (
	"integration_tests/tools/helpers"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IntegrationTests", func() {
	Describe("Web requests", func() {
		Describe("/", func() {
			It("returns 200 for healthchecks", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "") }).Should(Equal(true))

				session.Kill().Wait()
			})
		})

		Describe("/count", func() {
			It("increments the count, and returns 200", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "") }).Should(Equal(true))
				resp, err := http.Get("http://localhost:" + port + "/count")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				session.Kill().Wait()
			})
		})

		Describe("/status", func() {
			It("reports the current status of the emitters", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "") }).Should(Equal(true))

				By("returning non-200 when emitters are not done emitting")
				resp, err := http.Get("http://localhost:" + port + "/status")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusPreconditionFailed))

				By("incrementing the count of emitters finished")
				_, err = http.Get("http://localhost:" + port + "/count")
				Expect(err).ToNot(HaveOccurred())

				By("returning 200 when the number of finished emitters matches the instances")
				resp, err = http.Get("http://localhost:" + port + "/status")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				session.Kill().Wait()
			})
		})
	})
})
