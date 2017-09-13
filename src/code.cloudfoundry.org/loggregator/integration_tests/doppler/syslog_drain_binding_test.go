package doppler_test

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator/testservers"

	"github.com/cloudfoundry/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog Drain Binding", func() {
	Context("when connecting over TCP", func() {
		Context("when forwarding to an unencrypted syslog:// endpoint", func() {
			It("forwards log messages to a syslog", func() {
				etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
				defer etcdCleanup()
				dopplerCleanup, dopplerPorts := testservers.StartDoppler(
					testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
				)
				defer dopplerCleanup()
				ingressCleanup, ingressClient := dopplerIngressV1Client(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
				defer ingressCleanup()

				var drainServer *tcpServer
				Eventually(func() error {
					var err error
					drainServer, err = startUnencryptedTCPServer("localhost:0")
					return err
				}).ShouldNot(HaveOccurred())
				defer drainServer.close()
				syslogDrainAddress := fmt.Sprintf("localhost:%d", drainServer.port)

				syslogdrain := fmt.Sprintf(
					`{"hostname":"org.app.space.1","drainURL":"syslog://%s"}`,
					syslogDrainAddress,
				)
				key := drainKey("some-test-app-id", syslogdrain)
				etcdAdapter := etcdAdapter(etcdClientURL)
				addETCDNode(etcdAdapter, key, syslogdrain)

				Eventually(func() string {
					_ = sendAppLog("some-test-app-id", "syslog-message", ingressClient)
					line, err := drainServer.readLine()
					if err != nil {
						return ""
					}
					return line
				}, 20, 1).Should(ContainSubstring("syslog-message"))
			})
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			It("forwards log messages to a syslog-tls", func() {
				etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
				defer etcdCleanup()
				dopplerCleanup, dopplerPorts := testservers.StartDoppler(
					testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
				)
				defer dopplerCleanup()
				ingressCleanup, ingressClient := dopplerIngressV1Client(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
				defer ingressCleanup()

				var drainServer *tcpServer
				Eventually(func() error {
					var err error
					drainServer, err = startEncryptedTCPServer("localhost:0")
					return err
				}).ShouldNot(HaveOccurred())
				defer drainServer.close()
				syslogDrainAddress := fmt.Sprintf("localhost:%d", drainServer.port)

				syslogdrain := fmt.Sprintf(
					`{"hostname":"org.app.space.1","drainURL":"syslog-tls://%s"}`,
					syslogDrainAddress,
				)
				key := drainKey("some-test-app-id", syslogdrain)
				etcdAdapter := etcdAdapter(etcdClientURL)
				addETCDNode(etcdAdapter, key, syslogdrain)

				Eventually(func() string {
					_ = sendAppLog("some-test-app-id", "syslog-message", ingressClient)
					line, err := drainServer.readLine()
					if err != nil {
						return ""
					}
					return line
				}, 20, 1).Should(ContainSubstring("syslog-message"))
			})

			It("reconnects to a reappearing tls server", func() {
				etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
				defer etcdCleanup()
				dopplerCleanup, dopplerPorts := testservers.StartDoppler(
					testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
				)
				defer dopplerCleanup()
				ingressCleanup, ingressClient := dopplerIngressV1Client(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
				defer ingressCleanup()

				var drainServer *tcpServer
				Eventually(func() error {
					var err error
					drainServer, err = startEncryptedTCPServer("localhost:0")
					return err
				}).ShouldNot(HaveOccurred())

				syslogDrainAddress := fmt.Sprintf("localhost:%d", drainServer.port)
				syslogdrain := fmt.Sprintf(
					`{"hostname":"org.app.space.1","drainURL":"syslog-tls://%s"}`,
					syslogDrainAddress,
				)
				key := drainKey("some-test-app-id", syslogdrain)
				etcdAdapter := etcdAdapter(etcdClientURL)
				addETCDNode(etcdAdapter, key, syslogdrain)

				drainServer.close()

				Eventually(func() error {
					var err error
					drainServer, err = startEncryptedTCPServer(syslogDrainAddress)
					return err
				}).ShouldNot(HaveOccurred())

				Eventually(func() string {
					_ = sendAppLog("some-test-app-id", "syslog-message", ingressClient)
					line, err := drainServer.readLine()
					if err != nil {
						return ""
					}
					return line
				}, 20, 1).Should(ContainSubstring("syslog-message"))
			})
		})
	})

	Context("when forwarding to an https:// endpoint", func() {
		It("forwards log messages to an https endpoint", func() {
			etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
			defer etcdCleanup()
			dopplerCleanup, dopplerPorts := testservers.StartDoppler(
				testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := dopplerIngressV1Client(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
			defer ingressCleanup()

			logMessages := make(chan string)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				body, err := ioutil.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())

				logMessages <- string(body)
			}))

			syslogdrain := fmt.Sprintf(
				`{"hostname":"org.app.space.1","drainURL":"%s"}`,
				server.URL,
			)
			key := drainKey("some-test-app-id", syslogdrain)
			etcdAdapter := etcdAdapter(etcdClientURL)
			addETCDNode(etcdAdapter, key, syslogdrain)

			Eventually(func() string {
				_ = sendAppLog("some-test-app-id", "http-message", ingressClient)

				select {
				case msg := <-logMessages:
					return msg
				default:
					return ""
				}
			}, 20, 1).Should(ContainSubstring("http-message"))
		})
	})

	Context("when disabling syslog drains", func() {
		It("does not forward log messages to a syslog", func() {
			etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
			defer etcdCleanup()
			config := testservers.BuildDopplerConfig(etcdClientURL, 0, 0)
			config.DisableSyslogDrains = true
			dopplerCleanup, dopplerPorts := testservers.StartDoppler(config)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := dopplerIngressV1Client(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
			defer ingressCleanup()

			var drainServer *tcpServer
			Eventually(func() error {
				var err error
				drainServer, err = startUnencryptedTCPServer("localhost:0")
				return err
			}).ShouldNot(HaveOccurred())
			defer drainServer.close()
			syslogDrainAddress := fmt.Sprintf("localhost:%d", drainServer.port)

			syslogdrain := fmt.Sprintf(
				`{"hostname":"org.app.space.1","drainURL":"syslog://%s"}`,
				syslogDrainAddress,
			)
			key := drainKey("some-test-app-id", syslogdrain)
			etcdAdapter := etcdAdapter(etcdClientURL)
			addETCDNode(etcdAdapter, key, syslogdrain)

			Consistently(func() string {
				_ = sendAppLog("some-test-app-id", "syslog-message", ingressClient)
				line, err := drainServer.readLine()
				if err != nil {
					return ""
				}
				return line
			}).ShouldNot(ContainSubstring("syslog-message"))
		})
	})
})

func appKey(appID string) string {
	return fmt.Sprintf("/loggregator/v2/services/%s", appID)
}

func drainKey(appID string, drainURL string) string {
	hash := sha1.Sum([]byte(drainURL))
	return fmt.Sprintf("%s/%x", appKey(appID), hash)
}

func addETCDNode(etcdAdapter storeadapter.StoreAdapter, key string, value string) {
	node := storeadapter.StoreNode{
		Key:   key,
		Value: []byte(value),
		TTL:   uint64(20),
	}
	err := etcdAdapter.Create(node)
	Expect(err).NotTo(HaveOccurred())
	recvNode, err := etcdAdapter.Get(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(recvNode.Value)).To(Equal(value))
}
