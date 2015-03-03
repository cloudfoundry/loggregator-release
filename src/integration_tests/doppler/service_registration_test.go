package doppler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
    "github.com/onsi/gomega/gexec"
    "doppler/config"
    "time"
)

var _ = Describe("doppler service registration", func() {
	Context("with doppler healthy and running", func() {
		It("registers itself", func() {
			registration, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(registration.Value)).To(Equal(localIPAddress))
		})

		It("continues to register itself", func() {
			registration, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())
			initialModifiedIndex := registration.Index
			Consistently(func() string {
				registration, _ := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
				return string(registration.Value)
			}, time.Second + config.HeartbeatInterval).Should(Equal(localIPAddress))
			registration, err = etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(registration.Value)).To(Equal(localIPAddress))
			Expect(registration.Index).To(BeNumerically(">", initialModifiedIndex))
		})
	})

    Context("when doppler dies", func() {
      BeforeEach(func() {
          dopplerSession.Kill()
          Eventually(dopplerSession).Should(gexec.Exit())
      })

        It("stops registering itself", func(){
            Eventually(func() error {
                _, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
                return err
            }, time.Second + config.HeartbeatInterval).Should(HaveOccurred())
        })
    })
})
