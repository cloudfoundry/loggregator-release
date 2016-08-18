package debugtools_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Debugtools", func() {
	Describe("pprof", func() {

		Context("Doppler", func() {
			It("allows connection on pprofPort", func() {
				dopplerSession := startComponent(
					dopplerExecutablePath,
					"doppler",
					34,
					"--config=../fixtures/dopplertcp.json",
				)

				Eventually(func() bool { return checkEndpoint("6060", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				dopplerSession.Kill().Wait()
			})
		})

		Context("Traffic Controller", func() {
			It("allows connection on pprofPort", func() {
				tcSession := startComponent(
					trafficControllerExecutablePath,
					"tc",
					34,
					"--config=../fixtures/trafficcontroller.json",
					"--disableAccessControl",
				)

				Eventually(func() bool { return checkEndpoint("6060", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				tcSession.Kill().Wait()
			})
		})

		Context("Metron Agent", func() {
			It("allows connection on pprofPort", func() {
				metronSession := startComponent(
					metronExecutablePath,
					"metron",
					34,
					"--config=../fixtures/metrontcp.json",
				)

				// Wait for metron
				Eventually(metronSession.Buffer).Should(gbytes.Say("metron started"))

				Eventually(func() bool { return checkEndpoint("6061", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				metronSession.Kill().Wait()
			})
		})
	})

})
