package integration_tests_test

import (
	"bytes"
	"encoding/json"
	"integration_tests/tools/helpers"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IntegrationTests", func() {
	Describe("Web requests", func() {
		Describe("/", func() {
			It("returns 200 for healthchecks", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())

				session.Kill().Wait()
			})
		})

		Describe("/count", func() {
			It("increments the count, and returns 200", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())
				resp, err := http.Get("http://localhost:" + port + "/count")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				session.Kill().Wait()
			})
		})

		Describe("/status", func() {
			It("reports the current status of the emitters", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())

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

		Describe("/report", func() {
			It("accepts reports posted", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())

				By("returning StatusCreated when a report is posted to it")
				buf := bytes.NewBufferString(`
				{
					"errors": {
						"1008": 12,
						"other": 37
					},
					"messages": {
						"some-guid": {
							"app": "firstInstance",
							"total": 12,
							"max": 15
						},
						"some-other-guid": {
							"app": "secondInstance",
							"total": 13,
							"max": 15
						}
					}
				}
				`)

				resp, err := http.Post("http://localhost:"+port+"/report", "application/json", buf)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))
				Expect(resp.Header.Get("Location")).To(Equal("http://localhost:" + port + "/report"))

				session.Kill().Wait()
			})

			It("returns a compiled report", func() {
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())

				By("returning StatusCreated when a report is posted to it")
				buf := bytes.NewBufferString(`
				{
					"errors": {
						"1008": 12,
						"other": 37
					},
					"messages": {
						"some-guid": {
							"app": "firstInstance",
							"total": 12,
							"max": 15
						},
						"some-other-guid": {
							"app": "secondInstance",
							"total": 13,
							"max": 15
						}
					}
				}
				`)
				buf2 := bytes.NewBufferString(`
				{
					"errors": {
						"1008": 3,
						"other": 5
					},
					"messages": {
						"some-guid": {
							"app": "firstInstance",
							"total": 3,
							"max": 14
						},
						"some-other-guid": {
							"app": "secondInstance",
							"total": 1,
							"max": 16
						},
						"some-new-guid": {
							"app": "thirdInstance",
							"total": 1,
							"max": 3
						}
					}
				}
				`)

				resp, err := http.Post("http://localhost:"+port+"/report", "application/json", buf)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))

				resp, err = http.Post("http://localhost:"+port+"/report", "application/json", buf2)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))

				resp, err = http.Get(resp.Header.Get("Location"))
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				expected := map[string]map[string]interface{}{
					"errors": {
						"1008":  float64(15),
						"other": float64(42),
					},
					"messages": {
						"some-guid": map[string]interface{}{
							"app":   "firstInstance",
							"total": float64(15),
							"max":   float64(15),
						},
						"some-other-guid": map[string]interface{}{
							"app":   "secondInstance",
							"total": float64(14),
							"max":   float64(16),
						},
						"some-new-guid": map[string]interface{}{
							"app":   "thirdInstance",
							"total": float64(1),
							"max":   float64(3),
						},
					},
				}

				var body map[string]map[string]interface{}
				js := json.NewDecoder(resp.Body)
				Expect(js.Decode(&body)).To(Succeed())
				Expect(body).To(Equal(expected))

				session.Kill().Wait()
			})

			It("returns StatusPreconditionFailed if not all logcounters have checked in", func() {
				os.Setenv("COUNTER_INSTANCES", "2")
				session, port := helpers.StartLogfin()

				Eventually(func() bool { return helpers.CheckEndpoint(port, "", http.StatusOK) }).Should(BeTrue())

				By("returning StatusPreconditionFailed when not all have checked in")
				buf := bytes.NewBufferString(`
				{
					"errors": {
						"1008": 12,
						"other": 37
					},
					"messages": {
						"some-guid": {
							"app": "firstInstance",
							"total": 12,
							"max": 15
						}
					}
				}
				`)

				resp, err := http.Post("http://localhost:"+port+"/report", "application/json", buf)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))

				resp, err = http.Get(resp.Header.Get("Location"))
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusPreconditionFailed))

				By("returning StatusOK when all have checked in")
				buf2 := bytes.NewBufferString(`
				{
					"errors": {
						"1008": 3,
						"other": 5
					},
					"messages": {
						"some-guid": {
							"app": "firstInstance",
							"total": 3,
							"max": 14
						}
					}
				}
				`)

				resp, err = http.Post("http://localhost:"+port+"/report", "application/json", buf2)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))

				resp, err = http.Get(resp.Header.Get("Location"))
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				session.Kill().Wait()
			})
		})
	})
})
