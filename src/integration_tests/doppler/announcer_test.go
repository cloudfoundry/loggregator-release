package doppler_test

import (
	"doppler/config"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Doppler Announcer", func() {
	Context("with TLS enabled", func() {
		It("advertises udp, tcp, and ws endpoints", func() {
			node, err := etcdAdapter.Get("/doppler/meta/z1/doppler_z1/0")
			Expect(err).ToNot(HaveOccurred())

			expectedJSON := fmt.Sprintf(
				`{"version": 1, "endpoints":["udp://%[1]s:8765", "tcp://%[1]s:4321", "ws://%[1]s:4567", "tls://%[1]s:8766"]}`,
				localIPAddress)

			Expect(node.Value).To(MatchJSON(expectedJSON))
		})
	})

	Context("with TLS disabled", func() {
		var (
			dopplerSessionWithoutTLS *gexec.Session
			err                      error
		)

		BeforeEach(func() {
			command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler_without_tls.json")
			dopplerSessionWithoutTLS, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(dopplerSessionWithoutTLS.Out, 3).Should(gbytes.Say("Startup: doppler server started"))

			Eventually(func() error {
				_, err := etcdAdapter.Get("/doppler/meta/z1/doppler_z1/1")
				return err
			}, time.Second+config.HeartbeatInterval).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			dopplerSessionWithoutTLS.Kill().Wait()
		})

		It("advertises udp, tcp, and ws endpoints", func() {
			node, err := etcdAdapter.Get("doppler/meta/z1/doppler_z1/1")
			Expect(err).ToNot(HaveOccurred())

			expectedJSON := fmt.Sprintf(
				`{"version": 1, "endpoints":["udp://%[1]s:47654", "tcp://%[1]s:43210", "ws://%[1]s:45678"]}`,
				localIPAddress)

			Expect(node.Value).To(MatchJSON(expectedJSON))
		})

	})
})
