package app_test

import (
	"strings"

	"metron/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	It("IDN encodes DopplerAddrWithAZ", func() {
		c, err := app.Parse(strings.NewReader(`{
                "DopplerAddr": "doppler-addr",
                "DopplerAddrWithAZ": "jedinečné.doppler-addr:1234"
        }`))
		Expect(err).ToNot(HaveOccurred())

		Expect(c.DopplerAddrWithAZ).To(Equal("xn--jedinen-hya63a.doppler-addr:1234"))
	})

	It("strips @ from DopplerAddrWithAZ to be DNS compatable", func() {
		c, err := app.Parse(strings.NewReader(`{
                "DopplerAddr": "doppler-addr",
                "DopplerAddrWithAZ": "jedi@nečné.doppler-addr:1234"
        }`))
		Expect(err).ToNot(HaveOccurred())

		Expect(c.DopplerAddrWithAZ).ToNot(ContainSubstring("@"))
	})
})
