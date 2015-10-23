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
			dopplerSession.Interrupt().Wait(5 * time.Second)

			registration, err = etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
			Expect(err).To(HaveOccurred())

			registration, err = etcdAdapter.Get("/doppler/meta/z1/doppler_z1/0")
			Expect(err).To(HaveOccurred())
		})
	})
})
