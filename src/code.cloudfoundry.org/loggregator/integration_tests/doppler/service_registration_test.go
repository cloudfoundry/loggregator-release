package doppler_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("doppler service registration", func() {
	Context("with doppler healthy and running", func() {
		It("registers and unregisters itself", func() {
			registration, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(registration.Value)).To(Equal(localIPAddress))

			registration, err = etcdAdapter.Get("/doppler/meta/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(registration.Value)).NotTo(BeZero())

			By("dying")
			dopplerSession.Kill().Wait(5 * time.Second)

			fLegacy := func() error {
				_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
				return err
			}

			Eventually(fLegacy, 20).Should(HaveOccurred())

			f := func() error {
				_, err := etcdAdapter.Get("/doppler/meta/z1/doppler_z1/0")
				return err
			}
			Eventually(f, 20).Should(HaveOccurred())
		})
	})
})
