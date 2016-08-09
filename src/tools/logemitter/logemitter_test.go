package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Logemitter", func() {
	var (
		session *gexec.Session

		buildEnvVariables = func(envVariables map[string]string) []string {
			var result []string
			for name, value := range envVariables {
				result = append(result, fmt.Sprintf("%s=%s", name, value))
			}

			return result
		}

		startLogEmitter = func(envVariables map[string]string) {
			path, err := gexec.Build("tools/logemitter")
			Expect(err).ToNot(HaveOccurred())

			command := exec.Command(path)
			command.Env = buildEnvVariables(envVariables)

			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		}
	)

	Describe("LogFin connection", func() {
		Context("with a HTTPS connection that has an invalid cert", func() {
			var (
				logFinServer *httptest.Server
				rxCh         chan struct{}

				handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(rxCh)
				})
			)

			BeforeEach(func() {
				rxCh = make(chan struct{})
				logFinServer = httptest.NewTLSServer(handler)

				startLogEmitter(map[string]string{
					"LOGFIN_URL":       logFinServer.URL,
					"PORT":             "12345",
					"TIME":             "1ms",
					"SKIP_CERT_VERIFY": "true",
				})
			})

			AfterEach(func() {
				session.Kill().Wait(5 * time.Second)
				logFinServer.Close()

				gexec.CleanupBuildArtifacts()
			})

			It("utilizes the SKIP_CERT_VERIFY env variable", func() {
				Eventually(rxCh, 5).Should(BeClosed())
			})
		})
	})
})
