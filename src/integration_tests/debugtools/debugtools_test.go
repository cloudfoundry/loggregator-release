package debugtools_test

import (
	"doppler/dopplerservice"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/localip"
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
				// Wait for doppler to register
				key := fmt.Sprintf("%s/z1/doppler_z1/0", dopplerservice.LEGACY_ROOT)
				Eventually(func() bool {
					_, err := etcdAdapter.Get(key)
					return err == nil
				}, 5).Should(BeTrue())

				key = fmt.Sprintf("%s/z1/doppler_z1/0", dopplerservice.META_ROOT)
				Eventually(func() bool {
					_, err := etcdAdapter.Get(key)
					return err == nil
				}, 1).Should(BeTrue())

				resp, err := http.Get("http://localhost:6060/debug/pprof/heap")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				dopplerSession.Kill().Wait()
			})
		})

		Context("Traffic Controller", func() {
			It("allows connection on pprofPort", func() {
				LocalIPAddress, err := localip.LocalIP()
				Expect(err).ToNot(HaveOccurred())

				tcSession := startComponent(
					trafficControllerExecutablePath,
					"tc",
					34,
					"--config=../fixtures/trafficcontroller.json",
					"--disableAccessControl",
				)

				// Wait for traffic controller to startup
				waitOnURL("http://" + LocalIPAddress + ":49630")
				resp, err := http.Get("http://localhost:6060/debug/pprof/heap")
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
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

				//waitOnURL("http://" + LocalIPAddress + ":49625")
				Eventually(func() int {
					resp, _ := http.Get("http://localhost:6061/debug/pprof/profile")
					if resp != nil {
						return resp.StatusCode
					}
					return 503
				}).Should(Equal(200))
				metronSession.Kill().Wait()
			})
		})
	})

})
