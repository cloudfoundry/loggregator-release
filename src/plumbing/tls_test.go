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

		It("builds a config struct", func() {
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
	})

	Context("NewTLSConfig", func() {
		It("returns basic TLS config", func() {
			tlsConf := plumbing.NewTLSConfig()
			Expect(tlsConf.InsecureSkipVerify).To(BeFalse())
			Expect(tlsConf.ClientAuth).To(Equal(tls.NoClientCert))
			Expect(tlsConf.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConf.CipherSuites).To(ContainElement(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256))
			Expect(tlsConf.CipherSuites).To(ContainElement(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384))
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
MIIEJTCCAg2gAwIBAgIRAO/ptD//eUEdVZUcAiPL0+UwDQYJKoZIhvcNAQELBQAw
GDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xNTEwMjgyMTM2MDlaFw0xNzEw
MjgyMTM2MDlaMBExDzANBgNVBAMTBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAMcDTbJwxRZm/dHZ873ovldp06ZyjLI5zGgeEJhjteaTfFMR
tTtpdcZsIhZTldPOPadVieljQXAjPxE5j7X00CWmCGHxdpViCXGhxWuCRK6UOcgd
8C+cpHtsuqdcbJdyy1JrInpevS8ru1qDjqM7wfOFxrcfVjRsEwQg5FoIpLR37sH/
Q947rGNQAZfA6ny1M1zSn0qtT2JgxnZsNwTG81GOjBpCPsPfkq196Sx6/IUm03/f
Yp1178PGjIR3RcZkfrb3ao0xYYVpzCffXyidqK7ZNwYsOnQLGhSBH2smBUghzGIx
iH666AKqNvlGhnFge51/HdR40JrrRJuspCc+qCcCAwEAAaNxMG8wDgYDVR0PAQH/
BAQDAgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4EFgQU
39o9jPth1vAZy6C9+uEI6LRoIB4wHwYDVR0jBBgwFoAU0u5lHKjgZJMFvY/pVW+Q
UQsFdLowDQYJKoZIhvcNAQELBQADggIBAEa6G4n0XBpbeQLKmJ5CZSnpwajZwDR+
a1F6jY0N3ajbjTgKNUiSTTdQyWuOGqJZJyObcoaOgcce5YNWXH0Yq1Hzwyi1qoUk
sImomu7Tul6EqN1wfZ4ayvq20UpQAUHHXycIYnP7/NlqSpFm9PVJLcBd1EBxkcUN
WPoS8rVsWBiZ5rhC5cdPmOmjupWSNznU088cej3g8/tnmanukAYhv6do9w8xgcJx
4QtWFZ/Q2EZ4e2P8lf5D3R6KYiqPRcdGS+krwISizFpJmflKCdxzkihx4jSofM32
+2jFwpUQCJ9YDYDxIp1INIrD8HCFszZOthfLFA5uV9hlvpctQJHgCWwpJnczlo8o
zQ7T6z47cNVR6mrG0811u0GONZIkiAMNB48cWzLbsjPpbz3Sap9j4a5cHpIvabCf
pI7/nWmgOq5E3PGHkp+2RRJBdRVQNAmuKHr/tG02V9a627DhWVUr6JHtY7zY7ep0
BxDN66Vy/VXy0qOC7dZkww8XVoWoyf8ts9sRK/yhREpZ4n7pzD3r2+qb3PxmbfDI
2S7d/XpxrpC/Sll14ottCbONsBdf3jbV9d8WlW0rSfi7ZSgyBpBt+HNPrIbNXvm8
kjn0VhOd9baSX8A9GM2Hf65Gs2fy4NkPB56igI95eWjuDDzPhsjess8gYhaQPhVf
HpPEaHjzzIJP
-----END CERTIFICATE-----`

	clientKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAxwNNsnDFFmb90dnzvei+V2nTpnKMsjnMaB4QmGO15pN8UxG1
O2l1xmwiFlOV0849p1WJ6WNBcCM/ETmPtfTQJaYIYfF2lWIJcaHFa4JErpQ5yB3w
L5yke2y6p1xsl3LLUmsiel69Lyu7WoOOozvB84XGtx9WNGwTBCDkWgiktHfuwf9D
3jusY1ABl8DqfLUzXNKfSq1PYmDGdmw3BMbzUY6MGkI+w9+SrX3pLHr8hSbTf99i
nXXvw8aMhHdFxmR+tvdqjTFhhWnMJ99fKJ2ortk3Biw6dAsaFIEfayYFSCHMYjGI
frroAqo2+UaGcWB7nX8d1HjQmutEm6ykJz6oJwIDAQABAoIBAEmh90VmZAV95buX
II/LZWGCTkTvbQ8kQ3TiatF3Uv4U45L4ok1xH5pit9n64xyS2kznYTdw+e07nUIK
QhnYkorbe46BgJaUx1m7uQemEzNktFxOd2emVVU1TXpOv/7pAkFkUkVkeCrTy2YZ
9tR+b6xieruWZJbQxdhpMxP8zrPWZGJzamqbG0I2heMMsETpiVE1HhPTUQe92pnQ
5EcK6v7mIzp/Aot2bYsXmZl23Ln+9CWH9FOWeZUNlNmc8vDI9GrpMA4Izr3waa+a
9+ZM37aKzWwAF1b+p//Ukeg1zg+RvC8ksnh57sJ3inSy/ZkE5iZtfmBg859qThk5
llGiN4ECgYEA9gtJxI/qyv6ihHFPb2KPop0D5W7hxMrseQM20ZbFMqp5Sc8rYvDo
92wRSfj1RZUZdsMAC/305kLTYa1ZvN+V0Q/gFAMETCkiEELwRe5SXOS4jDp/1ri3
SZHUEEXID3Qirc2p21jkFpF8RtVeY7hiOfYM1/lBjSzJm3Jmib2BxF8CgYEAzxDU
nKJ2/qUaMEZB+p7mUrnpbFBWAWU2SKOm7mzq15tUwDejxSmcDdo+X1Z776+nhbz/
f4yOKIxGw2J00aVky1eN+U2xd5nyAqwK+cuEExSYY/IhFC32KIjLG9lbBbwDDKpP
vxVSpTkH9plVfOJaPif095V1YlbB/kKdhlHUcTkCgYAtqsKyXRPzQXfgpTddMSn/
wKzsdLwqzo89lr8h/53yXXnNnUosPxK+eaxr0m2T0Ky9QkxL7YL7CgQ56Pby+3zP
JOcuT7EIgcn0wrfeAvH+k+U9Ac6giABdA1gc/Ra455FYOQgB0mnjVnV+oDO4xoxU
vbp8i6MDFQEGfSFTB32CeQKBgCCcBO+6AkVuGOa7Wc6vUZR7pNAjArhriRX9d9+a
lY1o7/rpiEgXmnTwBtya0R/ZKOe98PrUVtr55HcGvWD6zBnd6wT1AFrWiq9zCrN2
IpGir7Elw6Ha7yZJDLuRCm2nw08uTyrHn+FXTvK+CSGGwDGDt2d6SSc4hIqXURmD
L5K5AoGAJf/elz3ub2GIHRkUPztv8vllpHykyHh+bvJiJY1yXZe2gYIQjSr8kCMy
jBaeRtxDZtG4jemGzGM5JNZ4FlONOWybszeCyhQswrpnxrS25+NsyBxXAQgSX/s+
2aumdeOue2gC5pBE9V1ctAx6g/mA6E1gK6xg/ZCxoraj/Oe1Kvc=
-----END RSA PRIVATE KEY-----`

	caCert = `
-----BEGIN CERTIFICATE-----
MIIFETCCAvmgAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDEw1sb2dn
cmVnYXRvckNBMB4XDTE1MTAyODIxMzYwN1oXDTI1MTAyODIxMzYwOVowGDEWMBQG
A1UEAxMNbG9nZ3JlZ2F0b3JDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoC
ggIBALOk6FybNAI363hhHJjOzTdPqPervNM1R+jXLP13BDs51lMLq5s6A/z0QWRa
M3x0vzNV+XKRMhxQXkcGDHRWccFMhhXRplq1M1MG25ds6FMyM5s2R5THzRBr8ZlO
g021PI61uBDIp12xLQIKovrZ/whW+YSpr/x/sW33UeLPkXIzMiAT6k9RMWwb8emI
Dt0WMRqzNyvwPwIOyVF4nVdjtpdStCk81JqXvZ99jKOISnw8oLaz5SK3iCQ11Csd
qsPmJOZHbHYVJ/C5Zce+sWXfaZRi2HZo+yGIlqHyesvN6kHfIdAUMxP9PbBpj2iY
4ihitBWcSu2nIPhMDIgYhJXcQuJm/ytbcOZ2IDrYjVIf6e8NtZnHcDpGtLGUH00G
qoXPBlxgf2uEiT03So7DbsHZMJ7Tp3OmenokDtV25T8ZPUgoZrb8aw/eLW4xVOWy
CbiAQLKPq7FljSHc18sl6zMfHrj31fKhKWS7oYKXEs2BXg9bDPNTlIkg8Q1QPrGF
v/2X3FG3FnBmhJptIaIjBawBsgCMFp/d5Sv3GH+sbrTKP9hRFWQ4f8mvSge2MoZb
A8CsKvSgrB63HM3I6PJX22Ym4XmQsHWo/6jhYhDHIC1+IwHYusv3Q8Ob62iVjTD0
FQ4mnVVTnmCpKsObwI1NTGO8HqDw81yOsnq7B8571I4OL4c7AgMBAAGjZjBkMA4G
A1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBTS7mUc
qOBkkwW9j+lVb5BRCwV0ujAfBgNVHSMEGDAWgBTS7mUcqOBkkwW9j+lVb5BRCwV0
ujANBgkqhkiG9w0BAQsFAAOCAgEAs3svK5++huGHZ3pQ2O4qFDtjtVWXX2gKVt5b
N8+H6CE8JwoqfmKdOvjjH+eCq09umOiAMDplNKt4jABQLpTRtpc+f/SoAiykf7lw
vdTfdQgZS/EMrtGtxvu4LrAOvBhfcq3nqzE/ybJ6IrPiy2Owt9T6B43g/S9h9ECC
n6CcA747s4WtJKi9g0/ezPswd2UQQ0yxpu8wetVPIRWRJchMVrpuSmm+TA9JpBag
lnPC7j7kpN2jOQDge4CR50lOhycfMlBCyU+i8ypg2b1lkY5HDaNjKcDwvQizWvqZ
qcR0M9vrqXjgvKBrZINtvgoYZtffZT7qtrqQBhWZ8l7xJ0lisoTVTu1y2xSDanMz
Jx+1ybelRZ/BSdDQa6kbrsvHIcVWcdiJrcYPPF4Yr1zX8erMDojI9lhDTXL2ptdh
x01lDTc/rUuYUUhYYjMHXLaINqQNP3mQr7V9iZs9PKHdU3FtrAmQf57hBHLy3mIB
1kSUlIslVUg086gejqvTO+aR5fYrQ4HnR7HD100AqHH2UORhwuF43xL2+IGz6YvF
p90nie0j6AMqteAJroHjPjxsvBwyQ/+YA4Crp6kYQXAnxlIBbpLpiCdrI92Bb9XB
6XB6kWdquC8zW+AZ2ev4bed0wfkgvb/j+/5bsLqYvR00CVg7UrnYlQW5/qsLy1Yc
bupooHk=
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
