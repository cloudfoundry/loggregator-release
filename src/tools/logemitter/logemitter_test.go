package main_test

import (
	"crypto/tls"
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
				crt, err := tls.X509KeyPair([]byte(tlsCrt), []byte(tlsKey))
				Expect(err).ToNot(HaveOccurred())

				logFinServer.TLS.Certificates = []tls.Certificate{crt}

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

const (
	tlsKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA6NUs1xBOR0Ei5SlZqx+tawgsdKRWB9iaOY+W9tugIxFgyF4A
U3VgJBjqZ0PAOrW9HToeO1gVL0F+T8tddUesKc7i1oUJeTV5fmIsfL1vj+DAfYln
Zeby2dUumdIxmhDi4zcXVQlQyJKAX4pkz6hVau+byp1cpZtAfTUqgmvYAkK7q/bx
2f8oFkQYp4q7MlXS7fphcHUE+5nIS0vU83S3pwsFlb5ThQqPwfvS+ghU7ZzjXhZJ
BUtZ75zO+Lz3GR16J6+NRZ5UJ/+A+bXgsiQj1I4lFJ+OPW2MBUmtMt/Fp1KGipd1
cJf5CtGoKIUOqYnUzPe4yrl6gI/rVdp6sA0UkwIDAQABAoIBAQCEJVmJxotnDaUM
g2eNJDF86eqxWQQq99iwirqX6Rb+UEKp9hAhTiD+29VOPrm/mJ55FB9MdzWu2HEk
QLwOcFtabz13m5JA5QTLolS1h57l/h3CIlY5E9cJo2ELlKzqUGM1qnLnpJ3g+KU7
lISbB2NTiiLV4HJQ28jCR4aU9zhmLvyyg4j7hg7krOIHqCFh1Mu9lguUihH5wg6j
rpndqK3JLXUpbLN6p7cy0r5TX6nb+flc0VgR+oi+zLj+ODtoeLkVuQfOTkdtz6Om
rjvxB2p+mKRiXCOqxO00W16xXR8ZEwXF9v45BQ18+oKRIEJdBbNWbsX+fiZhaZI5
saAIREKhAoGBAPqmoF4zAy2tnDN8c3L/R/gJpW1+YfmYzMZC4ZpCFoBciesi1xRU
c3mn8b+UJroVS5+5MR4zRmbppD5Sy0EADtJC2XsnLXeCGPj0xiFjPFiJ22zKgrQx
9XWjw+NqlpZzPyqEe4NnPQsG1HSuerHlpYED5U0K204qqlEpc8a80hs1AoGBAO3N
NAfAieC7cIqlgFQpUjNpWAAtuF8T7XZ/6djSjS+K2MUPGEVxieMEMeBCrzd+BUzY
73we5Gr2L65jQit689TALe99rgVyV/G3yPCQbk7l6tUs2EnZ2zHt8skPwS8M9/mn
J0lVZclqGOYz9SPvTlecdYZ/8UzoBo7WEZGbfaGnAoGBAMkF9B44iXcMAvej+y+i
n8TFb8CWGNvGeY0UvL0r/cHq9c34fkjWxlouoItGtZyOUb2DGGqhMvh8r/YwDsVN
15U4ehX0QNnVJFQec/z5Cr/zqGDjNdpxKuyzb/qnVKjLO0DNSgYEOYfrbV87RDoC
9S64wiF88JALVdeMCEe+zj91AoGAbEguUwFXRx/SxS9LWgdeyM5FJf+rno+iZ30j
bHmjlGxy/Hg9IxHqKZc1Ztq2klwt6ao2kpw2goYLfCrybH4WHWBNCmp+HTjN1uFK
/E+oCwEih2NeMXKkHv4suWUVIGmVWPbGKtxZ9vb604gBLhW/5KD32wDTTaOxqMTN
RzI4aK0CgYEAg3pvK94Jqj4gz25wHIi4+k//fIKPsUe05pjfQMPpW3UXW28+DshL
buWZd6Pt/fp9ZZ+yxKh7jfipDzYqZC2SlwqfWniz9NU9/+zPqbMDdu5GuBBylcAK
vLObG5KXgAnm+eKAA3rV911mIhXJWAop2R1GH+8t7un7Vmc4WUrqEmU=
-----END RSA PRIVATE KEY-----`
	tlsCrt = `-----BEGIN CERTIFICATE-----
MIIEIjCCAgqgAwIBAgIRAJWVlXj4VrkoLE2i8xIARNgwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEAxMHbG9jYWxDQTAeFw0xNjA0MzAyMzE3MDdaFw0xODA0MzAyMzE3
MDdaMBQxEjAQBgNVBAMTCTEyNy4wLjAuMTCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBAOjVLNcQTkdBIuUpWasfrWsILHSkVgfYmjmPlvbboCMRYMheAFN1
YCQY6mdDwDq1vR06HjtYFS9Bfk/LXXVHrCnO4taFCXk1eX5iLHy9b4/gwH2JZ2Xm
8tnVLpnSMZoQ4uM3F1UJUMiSgF+KZM+oVWrvm8qdXKWbQH01KoJr2AJCu6v28dn/
KBZEGKeKuzJV0u36YXB1BPuZyEtL1PN0t6cLBZW+U4UKj8H70voIVO2c414WSQVL
We+czvi89xkdeievjUWeVCf/gPm14LIkI9SOJRSfjj1tjAVJrTLfxadShoqXdXCX
+QrRqCiFDqmJ1Mz3uMq5eoCP61XaerANFJMCAwEAAaNxMG8wDgYDVR0PAQH/BAQD
AgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4EFgQU1+aP
J/E0HjVgRtnLur8/jCku6tgwHwYDVR0jBBgwFoAUKAF/qSAgNkaAY8eSRFVYEuZY
9vIwDQYJKoZIhvcNAQELBQADggIBAA8hvl2Fi/ca0gPVS2xvDCCmUlDpYLrpzyWy
uPbOlJrIgzmMVFkm/ccuifYtH2/KJ0eLI0ZXSHviTUiVPRk4325z176ubSLHQX96
8SjER3fx+57oy+Mln+qHW1jNjWJo8hhClbEEIGnHZxcGmgReODZtze8tNOPxWlsL
xmGp30XTWzTt6L8IwHnse5vfu4SSTdvarTxQoGr7UMruecfur+bV6R2+U4qXNvU5
Ffo5K/22yx/YmTux6XPRd10pI2T/rATP0WXC06oFsclpyUn3VX7ZKZeOyxSsGWZX
P0rhhnyFy0FmC4SJTAAFGvNe4xlivpBoyW9s5V/YV1Pkj2LqDVsm6YZeiGAP4n8j
kgPT/NKXmfab/uiyTA6sjWCyJ471X/qkttqI+FyBSm/OHhruStSR55Fa4+VQzTk5
+opasCyfWSNaLbkMXFXIB/tipIugMQp/fJSaXL3crCpm4s4Ggxu3btHOfxu+LBQi
TqDPiS7mGp5uMX1OiXyJv2xIbQqqv8NoUZZueUNQXHWoR6lafeF5tgCagRXizD2J
jDFhnGq0laV0qEZi2GG6OyV29jmzGq0o1ClzRYKBr4N97aFRO36piqMZhNCJSgpo
QZcBbcN/TaBy99Xmh5JA/r5VitWBSaQcSDnuCazhJ7mcQrhgeCq9ZKRk06bktrfE
OumLVWLz
-----END CERTIFICATE-----`
)
