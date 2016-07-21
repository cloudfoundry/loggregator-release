package integration_tests_test

import (
	"net/http"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("IntegrationTests", func() {
	Describe("Web requests", func() {
		Describe("/", func() {
			It("returns 200 for healthchecks", func() {
				session, port := startLogfin()

				Eventually(func() bool { return checkReady(port) }).Should(Equal(true))

				session.Kill().Wait()
			})
		})

		Describe("/count", func() {
			It("increments the count, and returns 200", func() {
				session, port := startLogfin()

				Eventually(func() bool { return checkReady(port) }).Should(Equal(true))
				resp, err := http.Get("http://localhost:" + port + "/count")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				session.Kill().Wait()
			})
		})

		Describe("/status", func() {
			It("reports the current status of the emitters", func() {
				session, port := startLogfin()

				Eventually(func() bool { return checkReady(port) }).Should(Equal(true))

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

func startLogfin() (*gexec.Session, string) {
	port := "8080"
	os.Setenv("PORT", port)
	os.Setenv("INSTANCES", "1")
	//PORT
	return startComponent(
		logfinExecutablePath,
		"logfin",
		34,
	), port
}

func checkReady(port string) bool {
	resp, _ := http.Get("http://localhost:" + port + "/")
	if resp != nil {
		return resp.StatusCode == http.StatusOK
	}

	return false
}
