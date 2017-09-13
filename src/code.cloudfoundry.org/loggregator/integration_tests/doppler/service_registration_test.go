package doppler_test

import (
	"fmt"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("doppler service registration", func() {
	Context("with doppler healthy and running", func() {
		It("registers and unregisters itself", func() {
			etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
			defer etcdCleanup()
			config := testservers.BuildDopplerConfig(etcdClientURL, 0, 0)
			dopplerCleanup, _ := testservers.StartDoppler(config)
			etcdAdapter := etcdAdapter(etcdClientURL)

			var f interface{} = func() string {
				registration, err := etcdAdapter.Get("healthstatus/doppler/test-availability-zone/test-job-name/42")
				if err != nil {
					return ""
				}
				return string(registration.Value)
			}
			Eventually(f).Should(Equal("127.0.0.1"))
			f = func() string {
				registration, err := etcdAdapter.Get("/doppler/meta/test-availability-zone/test-job-name/42")
				if err != nil {
					return ""
				}
				return string(registration.Value)
			}
			expectedJSON := fmt.Sprintf(`{"version": 1, "endpoints":["ws://127.0.0.1:%d"]}`, config.OutgoingPort)
			Eventually(f).Should(MatchJSON(expectedJSON))

			dopplerCleanup()

			f = func() error {
				_, err := etcdAdapter.Get("healthstatus/doppler/test-availability-zone/test-job-name/42")
				return err
			}
			Eventually(f, 20).Should(HaveOccurred())
			f = func() error {
				_, err := etcdAdapter.Get("/doppler/meta/test-availability-zone/test-job-name/42")
				return err
			}
			Eventually(f, 20).Should(HaveOccurred())
		})
	})
})
