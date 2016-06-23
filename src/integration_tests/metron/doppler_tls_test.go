package integration_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	dopplerConfig "doppler/config"
	metronConfig "metron/config"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("communicating with doppler over TLS", func() {
	const (
		sharedSecret     = "very secret"
		availabilityZone = "some availability zone"
		jobName          = "integration test"
		jobIndex         = "42"
	)

	It("forwards messages", func() {
		rand.Seed(time.Now().UnixNano())

		By("compiling metron")
		metronPath, err := gexec.Build("metron", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(metronPath)

		By("compiling doppler")
		dopplerPath, err := gexec.Build("doppler", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(dopplerPath)

		By("compiling etcd")
		etcdPath, err := gexec.Build("github.com/coreos/etcd", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(etcdPath)

		By("starting etcd")
		etcdPort := rand.Intn(55536) + 10000
		etcdClientURL := fmt.Sprintf("http://localhost:%d", etcdPort)
		etcdDataDir, err := ioutil.TempDir("", "etcd-data")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(etcdDataDir)

		etcdCommand := exec.Command(
			etcdPath,
			"--data-dir", etcdDataDir,
			"--listen-client-urls", etcdClientURL,
			"--advertise-client-urls", etcdClientURL,
		)
		etcdCommand.Stdout = gexec.NewPrefixedWriter("[o][etcd]", GinkgoWriter)
		etcdCommand.Stderr = gexec.NewPrefixedWriter("[e][etcd]", GinkgoWriter)
		etcdSession, err := gexec.Start(etcdCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer etcdSession.Kill()

		By("waiting for etcd to respond via http")
		Eventually(func() error {
			req, reqErr := http.NewRequest("PUT", etcdClientURL+"/v2/keys/test", strings.NewReader("value=test"))
			if reqErr != nil {
				return reqErr
			}
			resp, reqErr := http.DefaultClient.Do(req)
			if reqErr != nil {
				return reqErr
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusInternalServerError {
				return errors.New(fmt.Sprintf("got %d response from etcd", resp.StatusCode))
			}
			return nil
		}, 10).Should(Succeed())

		By("starting doppler")
		dopplerOutgoingPort := rand.Intn(55536) + 10000
		tlsPort := rand.Intn(55536) + 10000
		Expect(err).ToNot(HaveOccurred())
		dopplerConf := dopplerConfig.Config{
			OutgoingPort:       uint32(dopplerOutgoingPort),
			EtcdUrls:           []string{etcdClientURL},
			EnableTLSTransport: true,
			TLSListenerConfig: dopplerConfig.TLSListenerConfig{
				Port:     uint32(tlsPort),
				CertFile: "../fixtures/server.crt",
				KeyFile:  "../fixtures/server.key",
				CAFile:   "../fixtures/loggregator-ca.crt",
			},
			MaxRetainedLogMessages:       10,
			MessageDrainBufferSize:       100,
			SinkDialTimeoutSeconds:       10,
			SinkIOTimeoutSeconds:         10,
			SinkInactivityTimeoutSeconds: 10,
			UnmarshallerCount:            5,
			Index:                        jobIndex,
			JobName:                      jobName,
			SharedSecret:                 sharedSecret,
			Zone:                         availabilityZone,
		}

		dopplerCfgFile, err := ioutil.TempFile("", "doppler-config")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(dopplerCfgFile.Name())

		err = json.NewEncoder(dopplerCfgFile).Encode(dopplerConf)
		Expect(err).ToNot(HaveOccurred())
		err = dopplerCfgFile.Close()
		Expect(err).ToNot(HaveOccurred())

		dopplerCommand := exec.Command(dopplerPath, "--config", dopplerCfgFile.Name())
		dopplerCommand.Stdout = gexec.NewPrefixedWriter("[o][doppler]", GinkgoWriter)
		dopplerCommand.Stderr = gexec.NewPrefixedWriter("[e][doppler]", GinkgoWriter)
		dopplerSession, err := gexec.Start(dopplerCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer dopplerSession.Kill()

		// a terrible hack
		Eventually(dopplerSession.Buffer).Should(gbytes.Say("doppler server started"))

		By("waiting for doppler to listen")
		Eventually(func() error {
			c, reqErr := net.Dial("tcp4", fmt.Sprintf(":%d", dopplerOutgoingPort))
			if reqErr == nil {
				c.Close()
			}
			return reqErr
		}, 3).Should(Succeed())

		By("starting metron")
		metronPort := rand.Intn(55536) + 10000
		metronConf := metronConfig.Config{
			Deployment:      "deployment",
			Zone:            availabilityZone,
			Job:             jobName,
			Index:           jobIndex,
			IncomingUDPPort: metronPort,
			EtcdUrls:        []string{etcdClientURL},
			SharedSecret:    sharedSecret,
			Protocols:       metronConfig.Protocols([]metronConfig.Protocol{"tls"}),
			TLSConfig: metronConfig.TLSConfig{
				CertFile: "../fixtures/client.crt",
				KeyFile:  "../fixtures/client.key",
				CAFile:   "../fixtures/loggregator-ca.crt",
			},
			MetricBatchIntervalMilliseconds:  10,
			RuntimeStatsIntervalMilliseconds: 10,
			EtcdMaxConcurrentRequests:        10,
			TCPBatchIntervalMilliseconds:     100,
			TCPBatchSizeBytes:                10240,
		}

		metronCfgFile, err := ioutil.TempFile("", "metron-config")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(metronCfgFile.Name())

		err = json.NewEncoder(metronCfgFile).Encode(metronConf)
		Expect(err).ToNot(HaveOccurred())
		err = metronCfgFile.Close()
		Expect(err).ToNot(HaveOccurred())

		metronCommand := exec.Command(metronPath, "--debug", "--config", metronCfgFile.Name())
		metronCommand.Stdout = gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter)
		metronCommand.Stderr = gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter)
		metronSession, err := gexec.Start(metronCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer metronSession.Kill()

		// an equally terrible hack
		Eventually(metronSession.Buffer).Should(gbytes.Say(" from last etcd event, updating writer..."))

		By("waiting for metron to listen")
		Eventually(func() error {
			c, reqErr := net.Dial("udp4", fmt.Sprintf(":%d", metronPort))
			if reqErr == nil {
				c.Close()
			}
			return reqErr
		}, 3).Should(Succeed())

		err = dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronPort), "test-origin")
		Expect(err).NotTo(HaveOccurred())

		By("sending messages into metron")
		sent := make(chan struct{})
		go func() {
			defer close(sent)
			for i := 0; i < 100; i++ {
				err = logs.SendAppLog("test-app-id", "An event happened!", "test-app-id", "0")
				Expect(err).NotTo(HaveOccurred())
			}
		}()

		By("reading messages from doppler")
		Eventually(func() ([]byte, error) {
			c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://localhost:%d/apps/test-app-id/recentlogs", dopplerOutgoingPort), nil)
			if err != nil {
				return []byte{}, err
			}

			_, message, err := c.ReadMessage()
			if err != nil {
				return []byte{}, err
			}
			return message, err
		}).Should(ContainSubstring("An event happened!"))

		<-sent
	})
})
