package plumbing_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"plumbing"
)

var _ = Describe("TLS", func() {

	Context("NewMutalTLSConfig", func() {
		var (
			clientCertFilename  string
			clientKeyFilename   string
			caCertFilename      string
			nonSignedCAFilename string
		)

		BeforeEach(func() {
			clientCertFilename = writeFile(clientCert)
			clientKeyFilename = writeFile(clientKey)
			caCertFilename = writeFile(caCert)
			nonSignedCAFilename = writeFile(nonSignedCACert)
		})

		AfterEach(func() {
			err := os.Remove(clientCertFilename)
			Expect(err).ToNot(HaveOccurred())
			err = os.Remove(clientKeyFilename)
			Expect(err).ToNot(HaveOccurred())
			err = os.Remove(caCertFilename)
			Expect(err).ToNot(HaveOccurred())
		})

		It("builds a config struct with default CIPHERs", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				clientCertFilename,
				clientKeyFilename,
				caCertFilename,
				"test-server-name",
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.Certificates).To(HaveLen(1))
			Expect(conf.InsecureSkipVerify).To(BeFalse())
			Expect(conf.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
			Expect(conf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))

			Expect(string(conf.RootCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))
			Expect(string(conf.ClientCAs.Subjects()[0])).To(ContainSubstring("loggregatorCA"))

			Expect(conf.ServerName).To(Equal("test-server-name"))
		})

		It("allows you to not specify a CA cert", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				clientCertFilename,
				clientKeyFilename,
				"",
				"",
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(conf.RootCAs).To(BeNil())
			Expect(conf.ClientCAs).To(BeNil())
		})

		It("returns an error when given invalid cert/key paths", func() {
			_, err := plumbing.NewMutualTLSConfig("", "", caCertFilename, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to load keypair: open : no such file or directory"))
		})

		It("returns an error when given invalid ca cert path", func() {
			_, err := plumbing.NewMutualTLSConfig(clientCertFilename, clientKeyFilename, "/file/that/does/not/exist", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to read ca cert file: open /file/that/does/not/exist: no such file or directory"))
		})

		It("returns an error when given invalid ca cert file", func() {
			empty := writeFile("")
			defer func() {
				err := os.Remove(empty)
				Expect(err).ToNot(HaveOccurred())
			}()
			_, err := plumbing.NewMutualTLSConfig(clientCertFilename, clientKeyFilename, empty, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to load ca cert file"))
		})

		It("returns an error when the certificate is not signed by the CA", func() {
			_, err := plumbing.NewMutualTLSConfig(clientCertFilename, clientKeyFilename, nonSignedCAFilename, "")
			Expect(err).To(HaveOccurred())
			_, ok := err.(plumbing.CASignatureError)
			Expect(ok).To(BeTrue())
		})

		It("accepts configuration for CIPHER suites", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			))
		})

		It("ignores garbage CIPHERs in favor of valid ones", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{
					"GARBAGE",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))
		})

		It("panics if no ciphers are provided", func() {
			Expect(func() {
				plumbing.NewMutualTLSConfig(
					testservers.Cert("doppler.crt"),
					testservers.Cert("doppler.key"),
					testservers.Cert("loggregator-ca.crt"),
					"test-server-name",
					plumbing.WithCipherSuites([]string{}),
				)
			}).Should(Panic())
		})

		It("maintains the order of the ciphers provided", func() {
			conf, err := plumbing.NewMutualTLSConfig(
				testservers.Cert("doppler.crt"),
				testservers.Cert("doppler.key"),
				testservers.Cert("loggregator-ca.crt"),
				"test-server-name",
				plumbing.WithCipherSuites([]string{
					"TLS_RSA_WITH_RC4_128_SHA",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				}),
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(conf.CipherSuites).To(Equal([]uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
		})
	})

	Context("NewTLSConfig", func() {
		It("returns basic TLS config", func() {
			tlsConf := plumbing.NewTLSConfig()

			Expect(tlsConf.InsecureSkipVerify).To(BeFalse())
			Expect(tlsConf.ClientAuth).To(Equal(tls.NoClientCert))
			Expect(tlsConf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConf.CipherSuites).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			}))
		})

		It("accepts configuration for CIPHER suites", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}),
			)

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			))
		})

		It("ignores garbage CIPHERs in favor of valid ones", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{
					"GARBAGE",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				}),
			)

			Expect(conf.CipherSuites).To(ConsistOf(
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			))
		})

		It("panics if no ciphers are provided", func() {
			Expect(func() {
				plumbing.NewTLSConfig(
					plumbing.WithCipherSuites([]string{}),
				)
			}).Should(Panic())
		})

		It("maintains the order of the ciphers provided", func() {
			conf := plumbing.NewTLSConfig(
				plumbing.WithCipherSuites([]string{
					"TLS_RSA_WITH_RC4_128_SHA",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				}),
			)

			Expect(conf.CipherSuites).To(Equal([]uint16{
				tls.TLS_RSA_WITH_RC4_128_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			}))
		})

	})
})

func writeFile(data string) string {
	f, err := ioutil.TempFile("", "")
	Expect(err).ToNot(HaveOccurred())
	_, err = fmt.Fprintf(f, data)
	Expect(err).ToNot(HaveOccurred())
	return f.Name()
}

var (
	clientCert = `
-----BEGIN CERTIFICATE-----
MIIEHDCCAgSgAwIBAgIRANw8R8RgJUnXb7O9hpiohKowDQYJKoZIhvcNAQELBQAw
GDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xNzAxMTMyMjM1MDlaFw0xOTAx
MTMyMjM1MDlaMBIxEDAOBgNVBAMTB2RvcHBsZXIwggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQDK1HhCsIddzi6+jTVKFPOApeFtIa/be5CoFq8SJDOE/VJI
wuxiZlL28qDLfYbgXpeQ84qql9B1t0G1Gyo00bhUvyJ9IUyTi5TO2mVl+o5be77D
GfNRX7j1kOYLUdre9Cn+zx3+r2vS2OY3b1M1Fj/eGp2mg+y/r19v0J/55SWy5myM
zb4kbSwkpQJl3KOXlLx18ahR/llg2ud1CzeY4PsN35exin7oiEJlCMcJoEYmVHX0
z8rn85fpey2aSOYL0dJiubLH3mRp7CuFhLS3YlF5Ybl6nDjYvUi/lLpZ6lDVgSiE
S/UBUT+XFgN2Uo4LP8xovEbmrk6eWC2n8WrseVHBAgMBAAGjZzBlMA4GA1UdDwEB
/wQEAwIDuDATBgNVHSUEDDAKBggrBgEFBQcDAjAdBgNVHQ4EFgQUjkACkbao99mK
QypuGHvzv2UblQUwHwYDVR0jBBgwFoAU2Z1b4JnAwPRnFc4EOPiMDhm6lZgwDQYJ
KoZIhvcNAQELBQADggIBACnJ0N25Cs3MvWs32kC8yxWBGFxeM0Kmyg50aeCPaZEk
U8TOZM9nDvcP76ojlu6FzCgkqOthrPZNRJ2V6gG+qgLoKMyAgqra/bDZdVKS+G/e
+4KPgRAPj5+CzzHPusLWWH4k/tsYvHzpxOym3988xPKslYjtBYHXH1xAsGFb3Inw
CvApt/Mo6Bcz6633b1kdfOYZeeCVhxW2dnIaqPw9MKRXKs1wHtpoSygNeYQeELDk
QFG/jiF2XTa0fA22QT60/wWe4z2lkjQawhNHl1jJVpNvhKgbwOWWrsBUpBzZLgG9
l0tKWd3UYaGih4Ghh1VI9WAfmajoPBzGoMc6P2BCF8SyrfHEq1JdSM1htpPw+JfW
qX2mxPKowqhwu9cC9TTU1/gHAXfcK2duCu9AiuMj3N+vQkkbuS34SCGp+cEelINL
bRY/YBSCDZzDY9pkMWeIsX+woJevlQbdtZMzW2Siv1vT1s1Un9Ih9M+kEpJENsqq
0I2zeKsF3SfA+SYzeltB82Q4HSysFMRalk8eLf7zxrBoLzSttzhLEFhWDtaU4V0F
Qz1LmT6nmEBjV0kWntpXHdznSPUd8tT5rSxSG8/0zflQPPRqCTOSER2v112nKVuR
x73zvvBznnTpoxxSYxuBOk/QHsW9urnyxhOkQxd8vVJEsxEX11KAhW1ZJZDaxHwy
-----END CERTIFICATE-----`

	clientKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAytR4QrCHXc4uvo01ShTzgKXhbSGv23uQqBavEiQzhP1SSMLs
YmZS9vKgy32G4F6XkPOKqpfQdbdBtRsqNNG4VL8ifSFMk4uUztplZfqOW3u+wxnz
UV+49ZDmC1Ha3vQp/s8d/q9r0tjmN29TNRY/3hqdpoPsv69fb9Cf+eUlsuZsjM2+
JG0sJKUCZdyjl5S8dfGoUf5ZYNrndQs3mOD7Dd+XsYp+6IhCZQjHCaBGJlR19M/K
5/OX6XstmkjmC9HSYrmyx95kaewrhYS0t2JReWG5epw42L1Iv5S6WepQ1YEohEv1
AVE/lxYDdlKOCz/MaLxG5q5Onlgtp/Fq7HlRwQIDAQABAoIBAEM/uPPblbYrY2I4
bV+3nJid5yUI00bBLFAe3UL1j8NbPknznu5tILrR7TAq2WpQ0o3zwZkYJryw2u9S
J2dF1Yj7qlK0lLAiyl7fGFl8fnCkkbYcR4lGZIu+1BcSt6/OYpIiV76WqPhKg+ID
XiIu01QvnO+VtAxF+C2ZjUEMkbhDiL+gXYw0d7CpZHFBt4JC+KPzQwodpYlc3VxB
WdCcwiOdUuzlGnu0biJlO6Q+2hLRL4pvBodiMo0ZzO5ZyFua4pDeB92gVWK3+kop
lthu93pedWjKdygjALeM1kQrqeMZ/7yI7NiRtnrpX9qkJyTelZkHnuj9qQkc09RN
0LUJfDECgYEA59xro2KZeubziG2a9DVqVjyphU3OAELbR9TNg3r1scJy82sG5QWG
DCde6qFvpKzthWcCT/S9whIRo8+1v7qymH0onmklwvEvVsKGXn3MToE4Ahkotehl
8170+oSqnykvR/R8OZ3OPfQvC/B1b6s41ddap02jIxp/C+tOChnN9Q0CgYEA3/JP
pBBNocs/NOAdS3oHfe3Vkt2JmNTUq+adgoRpStJjPtiUpPy3hOVqc9o3mgU7yqhH
4O0BMkaH+C6dZCdCbV98gB2jTl1IT+Nt61HGjWjjlW9s8CJmGPGTsvSrvcyOKjym
X91BTOmm2tKw1aXP6h6RcfkCSVMrLcOj/Z/gioUCgYBkbQJqODDGHPZqpx6wm9o9
E/VQ+cw6LLsRt8h0JHP61IA5kqff1q6i4QKpmdbjess+Nsm3nAf84Rqm2zabnt/w
UHWhd2WVtCWO7J6Kmu49Kpb5wa/yaoCOExkE0SWd3pbOEcUkp4dHKlaeUz5qab0q
Uia/xE7ey4EvxnF8yoR8mQKBgFsANPqfIVy4oYOT+nN8L+UtKxdV7J7tBUqhGKo7
simUWn3kNmrgwpY/P0W6i9OLguN0BFlTFaRfYssn7g8PoP+eyJGq+XxTjZng+f6g
qUU3NRu5PpRJ9iD2saULpWon4DErmhPkba+aVpIfAXqfuWAScdnVbOds42PiVxYt
zGmdAoGBAIrxHkJ6OP2E22zMNyRU97XO0yosrTm81a5o7655xpIb5IS3n+yWP9Fo
gwNGx7NzqT5W+tvw01MXaznYisikrttX0K9e1raxzUVkeNwbX43xEH9zp/svNsC/
Vcqrx4A634dYYIVxAOACrlx6bokJdGjpKlxrG0RHiY7xKFj14QH+
-----END RSA PRIVATE KEY-----`

	caCert = `
-----BEGIN CERTIFICATE-----
MIIE8DCCAtigAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDEw1sb2dn
cmVnYXRvckNBMB4XDTE3MDExMzIyMzUwN1oXDTI3MDExMzIyMzUwOVowGDEWMBQG
A1UEAxMNbG9nZ3JlZ2F0b3JDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoC
ggIBALboZop6fMzg6Cb4f4nmAbwuk2QEUwkEyvj3BD9sxzinoUkCjEYWDYshVt7r
9R5WAJl4zEfpufTZWx/M9muGVIvSLc8j1rSMa2lmzkbhHayzG7EUI0UrqgWwe+NZ
QN24Ah6gDShUEj8ilN3szsMJwppNwYbJ/F+4PEwShcEAtr/eGwrEBTeS0COFuzAQ
sZUW+/jhjPahVIlChSoCuSaoBZWnwqdEyatjHNmbNH9E3fjvTjZcs9N6drqT43JC
DSeIRtPQHm59Rz89QzkCz9bifB//qnnE+iJS4CtIZYozrrvEXiCtWZArmQVys68w
cr5O3zH02qp6YJgVBajEp3bUo1rgUYRme8h8bn3GeEfGzYQBYe/MJtrHFItsZRXC
fslRuWlGiwwQVGwUI4m6d7zLbwdZY6bYhM3gBH0shop/daVkWxQF48vCEnwc4W1P
f4PMrQM4a9cz9iP1/0L77knCaYBgNjmFKGVDriMUvk9pgr7rUqBQx6tBtd+Zc+e+
jbJOj5mYRZX2aTy8w0NX0jExkaLv4V03tuqn4sMj28u16NgIlKCCxLuk/n1D6958
gQQ9lpuQirtxiFOVtKe2FS8YPTfe6sqFKcdjTyjDb8Pg8/Y/R+eFPvb6QuzgWIMn
UVTaHT1Lvd7RZpkAa01qaR84s9eQqwRhDpn2+yw2s4QanwD5AgMBAAGjRTBDMA4G
A1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBTZnVvg
mcDA9GcVzgQ4+IwOGbqVmDANBgkqhkiG9w0BAQsFAAOCAgEACTTkDUA/aZTLDm4P
eJye0NOR0hO2TulslC9Ahw8i1Wi/i1Hu9QeMW8zsi+ppwITNdradGstJrL/jhj5x
yGbGVFWYycnMpem2GxqNPN7jGPXkGJF3N97mrDR5QrrzeNW+ccfEvLf0PlIvF2lu
k/vwil6PuIDMh+fBFWfpiTALmgzOH0bhHrk3W8DYI60Cvu4T/+BXMz3SuEd4gnnu
xXGOtCxz6ZucPJJEV/PSiz0MBtBGFCLPOPrBHY3faesxOmuYCH08J+ADxe1ocbme
HXb2zFXd43p1b72zgJ8MXwggxKqNyOXH/GzYw5yrApvJ/PmX+5nnvqSs+QK+MAn1
+aZuERf2f7uyu5KozcV6VDrypGdagNpe3jZnBRJa0kY3GwdugjArelgk60glIrAw
0NaN+K9ntVGBzixLAzc+chXfllRNALy3ihKGC1RkbmevbmHeyTJcGr0fdL7+JacI
kiWW7CqGv7v4ytotNM3fZZ8EwqqxJ3KRCLm86VBcqpjKuijVJJy94Bh7vy2/JNit
lM502Apf+dUTbWwkwQ7QkfampZDBHOqitfbbcoVmpKMgS64AvczjlHZ7nYPKs+L2
+Ld3lgg8RNEFbcwVS9QeH5ZBW5LknktRMD+PpoPUsAU53c1tW0SPwJotG8YnZU70
8e++PmK5PpSwtV8H9mcJLdZWnoQ=
-----END CERTIFICATE-----`

	nonSignedCACert = `
-----BEGIN CERTIFICATE-----
MIIE8DCCAtigAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDEw1sb2dn
cmVnYXRvckNBMB4XDTE3MDEwNTE3MjQzMFoXDTI3MDEwNTE3MjQzNVowGDEWMBQG
A1UEAxMNbG9nZ3JlZ2F0b3JDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoC
ggIBAPLHojin/u+PZ+NWXrNFipHrMo8jc4J9UUB5udjteweNv6cGItZBybF12PrH
cuiYcajfpu3EBB0d2GjhwddGDhr9PCb8lIdkfGEjtwywdvnu4OcuuibP9vRHsVH8
01q5EYha9z37YXdVGSOa6fMBe/vVjX4q+N9xaGxVpxSJ8bbJj7TkMOOtk44X48xy
UuQsZAnidKXxDi873tpfDaX+V4ImUXeSQDvwOX05KSFEqqfNBXKQbIX+8vpO/Sb0
AUKbOEZvXsLLB6Q10Z+swbkARLqxZo77p9M9nxPugitdY694gzae/tduVto/az01
PHqduveI+nFq84T/4lhzLiO7cuG+hJMsbvbAiPuygxVh4e5divi2MzfMuvay70cs
xppPJ8kB6PLiGwN5aSyjuYBRcejQVTEQJeRdWSyiTknGllbOCgqd3hjKZ2qjE+es
6m+8KwgEKK+KSSnF1IiWxCe/eeYdvAzgaikB7Q6wJKZfqPdED9CpHm7yUL+ABhF3
GHR4QOwEtXzt4RgZ9ckr+pJjUmid1VXBqzp0/C47OlGFL47vCbEMvw1I4zFtWxnS
JuCKshAbwaVFF+XrbrimibBY4iSw7cxlcOBBLxfqjqFCmU80tNnQCAo5174su0HH
EIXNdAjb/Fr0CvTv6SYo44wfV4PJKOU9FLb3bHemUHbJLWAxAgMBAAGjRTBDMA4G
A1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBQ6n/UQ
PPjfmPnUJIz8VMzzns7KazANBgkqhkiG9w0BAQsFAAOCAgEAa0VGI85F7/qylHCt
QpcyW6P3PayUIhVO0YnJy9iWlSE6ecfZyqWmM/4WnaSfYH0Ak8uWoPo8WiFWJEl0
LLYWvRyZIwBjdqXgJ8ynCSaRawDacyFFu/fLhQ01KgsI2buMHRGab/HxcBcYDHq4
tf1RnlD8HVeGuZEBhu79BqUJucWlIC52evNShMWneCgOpfy2YoDX9mIMuh4Skmb+
nHDSo/0fa/X5vggltooeSLhCkesVSbb4RmctwPlpFFXKSBF6z2C8rFF5ouWeCQCk
yQYEODe5oRrx+hOJd7NCDzBtAMUfKRFHxhtL+sk3mCa9idyivi2UG5dmXDyIIrJ6
dvbXTFeG9XjyVWATsfUyQowslLvDCtobDiYcSNv94fwTD1+m/l4q8qRDMK4P0x8c
H5gnzWJ8chPbVy//XsOKv0jSHUjSFE/kWhwazevJGj4NjKgbxYEThRc73RDUp65t
rQ37g+d1vsPPBd9sM2wFSeJg64JPdlzxd0IhuEQoYmM0lQcUN3NYIzSvxx8SDYrO
c/yHffS6WmdY0ep8K2v1BH1PAyYBINPI5RFU2DAZGd0fUC+ryDWRvCpNaDYGHOJ+
i9ulkuurJM524mRT51gX2DrTbuJ2xCTY536bmPJmVFmfWaB3Rz+lt0dBMmlFRtEr
I1L4StQ7QGOszvGnFF/CcZnzwIY=
-----END CERTIFICATE-----
`
)
